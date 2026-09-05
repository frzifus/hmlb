# OpenEBS Mayastor XFS Journal Corruption

## Quick reference

A pod is stuck in `ContainerCreating` with `MountVolume.MountDevice failed ... Invalid argument (os error 22)`.
Mayastor pools are Online, nexus children are `healthy`. The XFS journal on the
volume is corrupt. Fix with `xfs_repair -L` via a manual NVMe-oF connect
through the Mayastor nexus — **no node reboot required**.

```bash
# on the nexus node (via a privileged toolbox pod):
xfs_repair -n /dev/nvmeXn1   # dry run first — always
xfs_repair -L /dev/nvmeXn1   # zero the corrupt journal
```

Nexus mirrors the repair writes to all replicas — one pass fixes them all.

## Symptoms

- Pod stuck in `ContainerCreating`, repeating:
  ```
  Warning  FailedMount  MountVolume.MountDevice failed for volume "pvc-<uuid>":
    rpc error: code = Internal desc = Failed to stage volume <uuid>:
    failed to mount device /dev/nvmeXn1 onto
    .../io.openebs.csi-mayastor/<hash>/globalmount: Invalid argument (os error 22)
  ```
- Mayastor pools report Online, zero I/O errors, nexus children `healthy: true`.
- Re-scheduling to another node fails identically (corruption travels with the replica data).

### Decisive evidence (`dmesg` on the node hosting the pod)

```
XFS (nvmeXn1): Corruption warning: Metadata has LSN (C:NNNNN) ahead of current LSN (C:NNNNN).
               Please unmount and run xfs_repair (>= v4.3) to resolve.
XFS (nvmeXn1): log mount/recovery failed: error -22
XFS (nvmeXn1): log mount failed
```

## Root cause

XFS **journal corruption** on the volume — *not* an OpenEBS / pool / replica /
node problem. After an unclean shutdown of the Mayastor io-engine (hard node
power-off, OOM-killed io-engine, forced pod eviction), the XFS log on a
thin-provisioned replica is left in a state where the filesystem metadata
references log sequence numbers **newer than the journal tail**. The kernel
refuses to replay the journal and fails the mount.

### Do NOT mistake it for

- **A "stale/leaked NVMe device".** The kernel reuses a freed controller index
  sequentially, so multiple failing volumes can all show up as `/dev/nvme8n1`.
  That is normal index reuse, not a leaked device. `dmesg` shows a valid XFS
  filesystem with the *correct* UUID failing log replay — proof the device is
  the right volume, just with a corrupt journal.
- **A pool/replica failure.** Pools report Online with zero I/O errors; the
  nexus reports both children `healthy: true`. The volume's `clean_shutdown:
  false` flag (in Mayastor etcd) is the real clue.
- **A node problem fixable by rebooting.** Rebooting does **not** help — the
  corrupt journal lives on the Mayastor replicas and survives a reboot. Don't
  reboot unless you've exhausted this runbook.

## Fix

### Prerequisites

- `kubectl` access to the cluster.
- `talosctl` is **not** required. Do **not** restart any node.
- You need the **volume UUID**, the **nexus node** (where the failing pod is
  scheduled), and that node's **internal IP**.

```bash
NAMESPACE=<app-namespace>          # e.g. vaultwarden
PVC=<pvc-name>                     # e.g. data-vaultwarden-0
VOLID=$(kubectl -n "$NAMESPACE" get pvc "$PVC" -o jsonpath='{.spec.volumeName}' | sed 's/^pvc-//')
NEXUS_NODE=$(kubectl -n "$NAMESPACE" get pod -l <pod-label> -o jsonpath='{.items[0].spec.nodeName}')
NEXUS_IP=$(kubectl get node "$NEXUS_NODE" -o jsonpath='{.status.addresses[?(@.type=="InternalIP")].address}')
echo "VOLID=$VOLID NEXUS_NODE=$NEXUS_NODE NEXUS_IP=$NEXUS_IP"
```

### Step 1 — Keep the consumer pod staging

The Mayastor nvmf frontend is **only published while a pod is actively
staging**. Keep the failing pod at 1 replica in `ContainerCreating` — its
retry loop keeps the frontend up.

```bash
kubectl -n "$NAMESPACE" scale <workload> --replicas=1
# wait for ContainerCreating + FailedMount events to appear
```

If the pod has been failing for days, the kubelet may back off its retry
interval to several minutes. The frontend stays published between retries.
If it isn't published (next step's `nvme connect` returns
`Connect Invalid Data Parameter`), delete the pod to force a fresh staging
attempt: `kubectl -n "$NAMESPACE" delete pod <pod>`.

### Step 2 — Launch a privileged toolbox pod on the nexus node

```bash
cat >/tmp/xfs-debug-pod.yaml <<EOF
apiVersion: v1
kind: Pod
metadata: { name: xfs-debug, namespace: openebs }
spec:
  nodeName: $NEXUS_NODE
  hostNetwork: true
  hostPID: true
  restartPolicy: Never
  containers:
  - name: toolbox
    image: docker.io/library/alpine:3.20
    command: ["sleep", "infinity"]
    securityContext: { privileged: true, runAsNonRoot: false }
    volumeMounts:
    - { name: dev,  mountPath: /dev }
    - { name: sys,  mountPath: /sys }
    - { name: proc, mountPath: /hostproc }
  volumes:
  - { name: dev,  hostPath: { path: /dev } }
  - { name: sys,  hostPath: { path: /sys } }
  - { name: proc, hostPath: { path: /proc } }
  tolerations: [ { operator: Exists } ]
EOF
kubectl apply -f /tmp/xfs-debug-pod.yaml
kubectl -n openebs wait --for=condition=ready pod/xfs-debug --timeout=120s
kubectl -n openebs exec xfs-debug -- sh -c 'apk add -q nvme-cli xfsprogs util-linux'
```

### Step 3 — Connect NVMe-oF and run dry-run

Connect to the volume's nexus frontend using the nexus node's own hostnqn
(it is in the nexus `allowed_hosts`). This creates a controller **separate
from the CSI's**, which the CSI's mount-failure cleanup won't tear down.

Gotcha: the CSI's staging retry disconnects *all* controllers for that
hostnqn roughly every ~8s. Run `xfs_repair` immediately after connecting.
If the device is yanked mid-repair just reconnect and try again — it converges.

```bash
NQN=nqn.2019-05.io.openebs:$VOLID
HOSTNQN=nqn.2019-05.io.openebs:node-name:$NEXUS_NODE
```

**Dry run first** — always. `xfs_repair -n` is read-only and tells you whether
metadata is clean (only the journal is bad → `-L` is safe and near-lossless) or
whether there is real metadata damage (→ `-L` would drop more; stop and weigh options).

```bash
kubectl -n openebs exec xfs-debug -- sh -c "
NQN=$NQN HOSTNQN=$HOSTNQN IP=$NEXUS_IP
for i in 1 2 3 4 5 6 7 8; do
  nvme disconnect -n \"\$NQN\" 2>/dev/null; sleep 0.3
  nvme connect -t tcp -a \"\$IP\" -s 8420 -n \"\$NQN\" -q \"\$HOSTNQN\" 2>/dev/null
  dev=\$(for n in /sys/class/nvme/nvme*; do [ -d \$n ] || continue
         [ \"\$(cat \$n/subsysnqn 2>/dev/null)\" = \"\$NQN\" ] && echo /dev/\$(basename \$n)n1; done)
  [ -b \"\$dev\" ] && { echo DRYRUN on \$dev; xfs_repair -n \"\$dev\" 2>&1 | tail -20; break; }
  sleep 1
done
nvme disconnect -n \"\$NQN\" 2>/dev/null
"
```

### Step 4 — Repair (`xfs_repair -L`)

`xfs_repair -L` zeros the corrupt journal and reformats the log. Because the
Mayastor nexus mirrors writes to **all** replicas, one repair through the
nexus fixes every replica — no need to repair each replica separately.

> **Data-loss note:** `-L` discards the uncommitted transactions still in the
> journal. In every case seen so far, the dry run showed the metadata itself
> was fully consistent — only the journal was bad — so `-L` lost effectively
> nothing.

```bash
kubectl -n openebs exec xfs-debug -- sh -c "
NQN=$NQN HOSTNQN=$HOSTNQN IP=$NEXUS_IP
for i in 1 2 3 4 5 6 7 8 9 10 11 12; do
  nvme disconnect -n \"\$NQN\" 2>/dev/null; sleep 0.3
  nvme connect -t tcp -a \"\$IP\" -s 8420 -n \"\$NQN\" -q \"\$HOSTNQN\" 2>/dev/null
  dev=\$(for n in /sys/class/nvme/nvme*; do [ -d \$n ] || continue
         [ \"\$(cat \$n/subsysnqn 2>/dev/null)\" = \"\$NQN\" ] && echo /dev/\$(basename \$n)n1; done)
  if [ -b \"\$dev\" ]; then
    echo REPAIR attempt \$i on \$dev
    xfs_repair -L \"\$dev\" >/tmp/r.log 2>&1; rc=\$?
    tail -7 /tmp/r.log; echo rc=\$rc
    [ \$rc -eq 0 ] && { echo REPAIR_OK; break; }
  else echo \"attempt \$i: no device (csi cycle)\"; fi
  sleep 1
done
nvme disconnect -n \"\$NQN\" 2>/dev/null
"
```

Success:
```
Maximum metadata LSN (C:NNNNN) is ahead of log (1:2).
Format log to cycle <N>.
done
rc=0
REPAIR_OK
```

## Post-fix checklist

1. `kubectl -n "$NAMESPACE" get pod -w` — pod moves from `ContainerCreating` to `Running`.
2. csi-node log: `"Volume <VOLID> staged to ..."` + `"Node Stage Volume Request completed successfully"`.
3. `dmesg` on the nexus node: `XFS (nvmeXn1): Ending clean mount`.
4. If the kubelet has backed off and isn't retrying, delete the pod to force a fresh stage.
5. Check `dmesg` on the node for **other** XFS `LSN ahead` lines — other volumes
   on the same node are often affected by the same io-engine event. Fix them the
   same way.

### Clean up

```bash
kubectl -n openebs delete pod xfs-debug --grace-period=0 --force
```

## Caveats

- **No backup fallback in this cluster.** PVCs labeled `k8up.io/backup: "true"`
  do not have a matching K8up `Schedule` unless one was created in their
  namespace. Don't assume a backup exists — check
  `kubectl -n <ns> get schedules.k8up.io`. Repair-in-place is usually the only option.
- **`repl: 1` vs `repl: 2`.** The corruption is in the journal on the served
  data. With `repl: 2` the replicas are mirrors and both carry the same corrupt
  journal — "delete one replica and rebuild from the other" does **not** help
  (the peer is equally corrupt). Repair through the nexus (which mirrors the
  repair writes) fixes both at once. With `repl: 1` there is only one replica;
  the same repair applies.

## Escalation

- **Dry run shows real metadata damage** (not just journal corruption) →
  `-L` would drop data. Stop, assess the volume's importance, and consider
  restoring from backup or recreating the workload with a fresh PVC.
- **`xfs_repair -L` keeps failing** after multiple connect/disconnect cycles →
  the device may have a hardware-level issue beyond journal corruption.
  Check the underlying disk health (`smartctl` on the storage node).
- **Multiple volumes affected** → fix them one at a time. Consider cordoning
  the node between repairs to avoid new staging attempts on unrepaired volumes.

## Prevention

No direct prevention exists — this is a consequence of unclean io-engine
shutdowns. Related mitigations:

- Ensure the node has a watchdog + panic-on-hang (see
  [storage1 node outage post-mortem](../post-mortems/2026-09-05-storage1-node-outage.md))
  so a wedged node reboots itself instead of hanging indefinitely.
- Use `repl: 2` on crucial volumes so a rebuild can restore data — but note
  this does **not** protect against journal corruption (both replicas are
  equally corrupt).

---

## Incident log

- **2026-08-25** — storage2 io-engine unclean shutdown corrupted the XFS
  journals of `vaultwarden` (`data-vaultwarden-0`, `pvc-a4df88a0`,
  openebs-cache, repl 2) and `azerothcore/mysql` (`pvc-7f79f1ae`,
  openebs-crucial, repl 1). Both recovered with `xfs_repair -L` via this
  procedure; no data loss (dry runs showed clean metadata). Plausible
  trigger: the cache-volume live-upgrade work from 2026-06-21.
