#!/usr/bin/env bash
# Show OpenEBS/Mayastor volume replication health.
# Flags volumes whose only healthy replica sits on an unhealthy pool
# (i.e. a dying disk) - those are one failure away from data loss.
#
# Usage: scripts/openebs-replica-status.sh [--all]
#   (default) only prints Degraded/at-risk volumes; --all prints every volume.
set -euo pipefail

SHOW_ALL="${1:-}"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

kubectl exec -n openebs deploy/openebs-mayastor-api-rest -c api-rest -- \
    wget -qO- "http://localhost:8081/v0/pools" > "$TMPDIR/pools.json"
kubectl exec -n openebs deploy/openebs-mayastor-api-rest -c api-rest -- \
    wget -qO- "http://localhost:8081/v0/volumes?max_entries=500" > "$TMPDIR/vols.json"
kubectl get pv -o json > "$TMPDIR/pv.json"

SHOW_ALL="$SHOW_ALL" python3 - "$TMPDIR/pools.json" "$TMPDIR/vols.json" "$TMPDIR/pv.json" <<'PY'
import json, os, sys

pools = json.load(open(sys.argv[1]))
vols  = json.load(open(sys.argv[2]))
pvs   = json.load(open(sys.argv[3]))
show_all = os.environ.get("SHOW_ALL") == "--all"

# pool id -> status; anything not Online is "unhealthy" (Degraded/Faulted/Suspected)
pool_status = {p["id"]: p.get("state", {}).get("status", "Unknown") for p in pools}
bad_pools = {pid for pid, st in pool_status.items() if st != "Online"}

# volume-handle (uuid) -> ns/name
pvc = {}
for pv in pvs["items"]:
    h = pv.get("spec", {}).get("csi", {}).get("volumeHandle", "")
    ref = pv["spec"].get("claimRef", {})
    if h and ref:
        pvc[h] = f'{ref.get("namespace")}/{ref.get("name")}'

if bad_pools:
    print("Unhealthy pools:", ", ".join(f"{p}({pool_status[p]})" for p in sorted(bad_pools)))
    print()

hdr = f'{"VOLUME":13} {"STATUS":9} {"SIZE":>7}  {"HEALTHY/TOTAL":13} {"HEALTHY_ON":22} {"FLAG":8} PVC'
print(hdr)
print("-" * len(hdr))

rows = []
for v in vols["entries"]:
    st = v.get("state", {})
    uuid = st.get("uuid", v["spec"]["uuid"])
    status = st.get("status", "?")
    gb = v["spec"]["size"] / (1024**3)
    rt = st.get("replica_topology", {})
    total = len(rt)
    healthy = [r for r in rt.values() if r.get("healthy")]
    healthy_pools = sorted({r["pool"] for r in healthy})
    # at-risk: every healthy replica lives on an unhealthy pool
    at_risk = healthy and all(r["pool"] in bad_pools for r in healthy)

    flag = ""
    if at_risk:
        flag = "AT-RISK"
    elif status != "Online":
        flag = "DEGRADED"

    if not show_all and not flag:
        continue
    rows.append((0 if at_risk else 1, uuid, status, gb, len(healthy), total,
                 ",".join(healthy_pools), flag, pvc.get(uuid, "-")))

for _, uuid, status, gb, nh, total, hp, flag, name in sorted(rows):
    print(f'{uuid[:12]:13} {status:9} {gb:6.0f}G  {f"{nh}/{total}":13} {hp:22} {flag:8} {name}')

atr = sum(1 for r in rows if r[7] == "AT-RISK")
print(f'\n{len(rows)} volume(s) shown; {atr} AT-RISK (only healthy copy on an unhealthy pool).')
PY
