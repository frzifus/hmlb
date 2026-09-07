# ClickHouse Zero-Byte MergeTree Parts (SigNoz on Talos)

## Quick reference

ClickHouse will not load MergeTree parts whose files are all zero bytes. When
enough parts are damaged, the server exits with code `210` and the pod
crash-loops:

```
Suspiciously many (N parts, ... in total) broken parts...
max allowed broken parts count is 100
```

Do not raise `max_suspicious_broken_parts` unless you explicitly accept
data loss. The safe recovery is to scale ClickHouse down, attach its PVC to a
repair pod, remove only part directories that contain no non-empty file, then
scale ClickHouse back up.

```sh
kubectl -n observability scale statefulset chi-backend-clickhouse-cluster-0-0 --replicas=0
kubectl -n observability wait --for=delete pod/chi-backend-clickhouse-cluster-0-0-0 --timeout=120s
# start chi-repair from below, count dry-run, then remove
kubectl -n observability delete pod chi-repair
kubectl -n observability scale statefulset chi-backend-clickhouse-cluster-0-0 --replicas=1
kubectl -n observability wait --for=condition=ready pod/chi-backend-clickhouse-cluster-0-0-0 --timeout=900s
```

## Symptoms

- `chi-backend-clickhouse-cluster-0-0-0` is `CrashLoopBackOff`; the
  `clickhouse` container exits `210`.
- Previous logs repeat `is broken and needs manual correction` and
  `CANNOT_PARSE_INPUT_ASSERTION_FAILED`.
- The log's part listing includes zero-size `data.bin`, `columns.txt`,
  `checksums.txt`, index files, and the message `Part is empty`.
- Storage is mounted and the filesystem is not out of space.
- The problem survives pod restarts because the corrupt part directories live
  on the PVC.

## Root cause

This is storage-level corruption surfaced by ClickHouse, not a bad SigNoz
chart. The Mayastor volume API showed:

```json
"health": {
  "cleanShutdown": false,
  "healthyReplicas": 2,
  "cleanReplicas": 1,
  "onlineHealthyReplicas": 2,
  "onlineCleanReplicas": 1
}
```

One replica was `OutOfSync`. Mayastor recreated the nexus with only one replica
and then performed a full 50 Gi rebuild. A rebuild copies blocks; it does not
reconstruct files already damaged by an interrupted or unclean write cycle.
That can leave MergeTree part directories on XFS where every file is zero
bytes.

ClickHouse can report only a subset of the bad parts before failing. In the
2026-09-07 incident, the volume held 3,324 part directories and 1,859 were
entirely zero-byte. Removing those directories made the volume loadable while
preserving the healthy parts.

## Diagnosis

Set the PVC and pod values for this installation:

```sh
NAMESPACE=observability
POD=chi-backend-clickhouse-cluster-0-0-0
STS=chi-backend-clickhouse-cluster-0-0
PVC=data-volumeclaim-template-chi-backend-clickhouse-cluster-0-0-0
```

Confirm the last termination and exit code:

```sh
kubectl -n "$NAMESPACE" get pod "$POD" -o jsonpath=\
'{range .status.containerStatuses[*]}{.name}{" exit="}{.lastState.terminated.exitCode}{" reason="}{.lastState.terminated.reason}{"\n"}{end}'
```

Look for the decisive ClickHouse evidence:

```sh
kubectl -n "$NAMESPACE" logs "$POD" -c clickhouse --previous --tail=500 | \
  rg 'Suspiciously many|broken parts|Part is empty|CANNOT_PARSE_INPUT'
```

Get the PVC/PV IDs and Mayastor health:

```sh
PV=$(kubectl -n "$NAMESPACE" get pvc "$PVC" -o jsonpath='{.spec.volumeName}')
VOLID=${PV#pvc-}
echo "PV=$PV VOLID=$VOLID"
kubectl -n openebs port-forward svc/openebs-mayastor-api-rest 18081:8081
curl -fsS "http://127.0.0.1:18081/v0/volumes/$VOLID" | jq '.state'
```

`healthyReplicas: 2` does not prove the data is clean. Check `cleanReplicas`
and whether any replica was `OutOfSync`.

## Repair

### 1 — Stop ClickHouse

```sh
kubectl -n "$NAMESPACE" scale statefulset "$STS" --replicas=0
kubectl -n "$NAMESPACE" wait --for=delete pod "$POD" --timeout=120s
```

Do not delete the PVC for this procedure. Deleting it is a full telemetry wipe
and should be a separate, explicitly approved recovery.

### 2 — Start a repair pod

The PVC must be detached before the repair pod can bind it. The volume
topology decides the repair pod's node; the `openebs.io/csi-node` selector only
restricts it to a Mayastor-capable node.

```sh
kubectl apply -n "$NAMESPACE" -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: chi-repair
spec:
  restartPolicy: Never
  nodeSelector:
    openebs.io/csi-node: mayastor
  containers:
    - name: repair
      image: docker.io/alpine:3.18.2
      command: ["/bin/sh", "-c", "sleep 3600"]
      volumeMounts:
        - name: data
          mountPath: /var/lib/clickhouse
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: $PVC
EOF

kubectl -n "$NAMESPACE" wait --for=condition=ready pod/chi-repair --timeout=120s
```

### 3 — Count zero-byte parts first

This is read-only. A part directory is a candidate only if it contains no file
larger than zero bytes.

```sh
kubectl -n "$NAMESPACE" exec pod/chi-repair -- sh -c '
  base=/var/lib/clickhouse/store
  total=0
  zero=0
  for dir in $(find "$base" -mindepth 3 -maxdepth 3 -type d); do
    total=$((total + 1))
    if ! find "$dir" -type f -size +0c -print -quit | grep -q .; then
      zero=$((zero + 1))
    fi
  done
  echo "total_parts=$total zero_byte_parts=$zero"
'
```

If `zero_byte_parts=0`, the crash-loop has a different cause; stop here.

### 4 — Remove zero-byte parts

The command records every removed path in
`/var/lib/clickhouse/removed-zero-byte-parts.txt`, then removes it. It never
touches a directory with non-empty files.

```sh
kubectl -n "$NAMESPACE" exec pod/chi-repair -- sh -c '
  set -e
  base=/var/lib/clickhouse/store
  removed=0
  total=0
  for dir in $(find "$base" -mindepth 3 -maxdepth 3 -type d); do
    total=$((total + 1))
    if ! find "$dir" -type f -size +0c -print -quit | grep -q .; then
      echo "$dir" >> /var/lib/clickhouse/removed-zero-byte-parts.txt
      rm -rf -- "$dir"
      removed=$((removed + 1))
    fi
  done
  echo "removed=$removed total_before=$total"
  du -sh "$base"
'
```

### 5 — Return to normal

```sh
kubectl -n "$NAMESPACE" delete pod chi-repair
kubectl -n "$NAMESPACE" scale statefulset "$STS" --replicas=1
kubectl -n "$NAMESPACE" wait --for=condition=ready pod "$POD" --timeout=900s
kubectl -n "$NAMESPACE" get pod "$POD" -o wide
kubectl -n "$NAMESPACE" logs "$POD" -c clickhouse --tail=500 | \
  rg 'Suspiciously many|broken parts|Part is empty' || true
```

The pod is fixed when it is `1/1 Running` and ready with no new
`Suspiciously many ... broken parts` lines.

## Post-fix checklist

1. Pod is ready and no longer exits `210`.
2. Recent logs no longer report empty MergeTree parts.
3. Query a few recent logs/traces/metrics through SigNoz to verify ingest.
4. Review Mayastor replica health and restore the desired replica count.
5. Watch the pod for one ingest interval; merge and memory warnings can remain
   after recovery and are not the original startup failure.

## Caveats

- If telemetry history matters, check for an available Mayastor snapshot or
  PVC backup before removal. The incident volume had no snapshots.
- Removing all-zero part directories is data loss, but it is bounded. In the
  2026-09-07 incident, 1,859 of 3,324 parts were removed and the remaining data
  stayed available.
- A healthy Mayastor rebuild is not data repair. If the surviving replica
  already contains zero-byte files, the rebuild propagates the same corruption.
- Do not run the removal while ClickHouse is writing. Scale ClickHouse down
  first and make sure the PVC is detached.
- If the pod fails at mount time with an XFS `Invalid argument` or
  `LSN ahead` error, use
  [OpenEBS Mayastor XFS Journal Corruption](openebs-mayastor-xfs-log-corruption.md)
  before using this runbook.

## Escalation

- **Zero-byte part count is small but ClickHouse still will not start** → stop
  and inspect the exact table/part errors before deleting anything else.
- **The volume has no healthy clean replica** → stop, assess whether backup or
  full reset is safer, and involve storage support.
- **Merge or memory errors continue to grow after recovery** → check SigNoz
  retention and ClickHouse memory limits; do not confuse those with this
  corruption case.

## Incident log

### 2026-09-07 00:43 UTC

`chi-backend-clickhouse-cluster-0-0-0` had restarted 41 times and exited `210`
with the suspicious-broken-parts limit. The ClickHouse volume was a 50 Gi
Mayastor PVC on `gpu2` (`VOLID=12404a3d-c5f3-44bd-b05a-fdd7145d032e`).

The Mayastor state showed `cleanShutdown: false` and `cleanReplicas: 1/2`; one
replica had been `OutOfSync`. A node-debug scan found 3,324 part directories,
1,859 of them entirely zero-byte. Scaling down ClickHouse, removing those
directories through `chi-repair`, and scaling it back up restored the pod to
`1/1 Running` without deleting the PVC.
