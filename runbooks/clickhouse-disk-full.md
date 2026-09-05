# ClickHouse Disk-Full Runbook (SigNoz on Talos)

## Quick reference

At 100% full, `TRUNCATE` / `ALTER` / `DROP PARTITION` all fail (code 243) —
ClickHouse needs headroom for metadata ops. Drop whole system.* log tables
instead (they auto-recreate in ~1 min):

```sql
DROP TABLE system.trace_log;
DROP TABLE system.query_log;
```

Returns EOF/500 to the MCP caller **on success**. Always re-check with:

```sql
SELECT formatReadableSize(free_space) FROM system.disks;
```

Double-drop is safe — errors with 'table does not exist'.

## Symptoms

- Disk at/near 100% (`system.disks.free_space` → 0)
- SigNoz API timeouts, MCP cooldowns
- Code 241 (memory limit exceeded) or code 243 (cannot reserve) in text_log
- Nodata alerts (`[No data] 0%`) that don't match reality — check `system.disks` directly

## Diagnosis

### Check disk

```sql
SELECT name, formatReadableSize(total_space) total, formatReadableSize(free_space) free FROM system.disks;
```

### Find the biggest tables

```sql
SELECT database, table, formatReadableSize(sum(bytes_on_disk)) s
FROM system.parts
WHERE active
GROUP BY database, table
ORDER BY sum(bytes_on_disk) DESC;
```

### Check TTLs

```sql
SELECT name, engine_full FROM system.tables WHERE database='system';
```

### MCP syntax notes

SQL via builder v5: `{spec: {query: "<sql>"}, type: "clickhouse_sql"}` inside
`compositeQuery.queries`. `type: "sql"` returns 400. `requestType: scalar`.
`start`/`end` required (any 1h window works for `system.*` queries).
Scalar queries need `reduceTo` inside each aggregation entry.
`EXCEPTIONS_BASED_ALERT` is the accepted alertType for `clickhouse_sql` rules.

### Key table facts

- `system.parts` has no `event_date` — use `min_date` / `max_date`.
- Traces live in `signoz_traces.signoz_index_v3` (`signoz_spans` is legacy/empty).
  `index_v3.timestamp` is UInt32 seconds → `toDateTime(max(timestamp))` directly.
- SigNoz data age: `logs_v2.timestamp` = DateTime64(9) NANOS;
  `traces.timestamp` = DateTime64(9) SECONDS; `samples_v4.unix_milli` = ms.
  Use `intDiv(col, unit)` to seconds, then `toDateTime`.

## Immediate fix

At 100% full, drop the heaviest `system.*` log tables:

```sql
DROP TABLE system.trace_log;
DROP TABLE system.query_log;
```

- All `system.*` log tables auto-recreate without restart (~1 min; `part_log`
  may resist — retry or wait for TTL).
- `trace_log` freed 24 GiB instantly; 0 → 38 GiB free after `trace_log` + `query_log`.
- If a TTL purge storm is saturating CH (see [TTL purge OOM storm](#ttl-purge-oom-storm)),
  `DROP TABLE system.metric_log` (only ~0.7 GiB on disk; the 10.9k small parts
  are the merge fuel) — auto-recreates, TTL reverts to 30d → immediately re-apply.

## Post-fix checklist

1. `system.disks` free space is reasonable (not 0).
2. Re-apply TTLs on auto-recreated tables (see below) and verify `engine_full`.
3. Storm-over check: `SELECT count() FROM system.text_log WHERE event_time > now()-60 AND message LIKE '%Code: 241%'` == 0.
4. Fresh data is landing: check `system.processes` for small healthy inserts.
5. Alerts inactive (or firing for a real reason, not nodata).

## TTL management

### System tables (must re-apply after any DROP)

Auto-recreated tables revert to config defaults (usually 30d). Re-apply via
`ALTER TABLE ... MODIFY TTL` (500/EOF response = success through the MCP path;
always verify `engine_full` after):

- `query_log`, `part_log`: 3d
- `metric_log`, `asynchronous_metric_log`, `query_views_log`, `session_log`, `zookeeper_log`: 2d
- `processors_profile_log`, `trace_log`: 1d
- `text_log`: already had 3d (verify, don't assume)

### Caveats

- **MATERIALIZE TTL** on a 10 GiB+ table OOMs (code 241, 5.4 GiB server limit)
  and hogs memory for ~30 min. `KILL MUTATION` is async ('waiting' is normal).
- Fresh TTL applies to new partitions automatically — old partitions purge
  partition-wise without materialize.
- `queryService.configVars.*_ttl_duration_hrs` keys in the SigNoz HelmRelease
  ([`clusters/homelab/core/observability/signoz.yaml`](../clusters/homelab/core/observability/signoz.yaml))
  are **dead** in chart 0.139.0 (deployed STS has no TTL env vars) —
  retention is the SigNoz setTTL API, not helm values.

### signoz_* retention

Lowering signoz logs/traces retention does **not** help the disk: `signoz_*`
is ~4.5 GiB (logs 174 MiB, traces 26 MiB, metrics 3.2 GiB) on a 50 GiB
volume; system tables are the eater. No setTTL tool in the signoz MCP
catalog; app-level settings API not reachable from the sandbox (netpol).
Retention for `signoz_*` tables = the `_retention_days` column baked into
each table's `PARTITION BY` + TTL expression (see `engine_full`; change
via UI/API only).

## TTL purge OOM storm

A TTL-purge backlog can saturate CH **without filling the disk**. When
`metric_log` TTL was lowered 30d→2d, the purge merged 10.9k source parts
from Aug-20 partitions, OOMed at the 5.4 GiB server limit, and retried
infinitely (~20 code-241 errors/sec for 15+ min, 60 errors per 3s window).

Symptoms:
- SigNoz API timeouts, MCP cooldowns
- Alert evaluations missed → nodata alert fires while disk is actually fine
- `system.parts` per table: `max(partition)` < today on a TTL'd table = purge stuck

Fix:
```sql
DROP TABLE system.metric_log;
```
Then immediately re-apply the 2d TTL and verify `engine_full`.

Storm-over check:
```sql
SELECT count() FROM system.text_log WHERE event_time > now()-60 AND message LIKE '%Code: 241%';
```

## Alerts

### ClickHouse Disk Space (01a0672b-df88-77b0-b23d-ae815fa497f0)

`clickhouse_sql` on `system.disks` free %. Warning <20 / critical <10 with
hysteresis, `alertOnAbsent` 15m, 10m window.

### PVC almost full (019d45f6)

groupBy `k8s.volume.name` (not `k8s.persistentvolumeclaim.name` — that key
does not exist on `k8s.volume.*`), 80/90% tiers + 75/85 hysteresis, 10m
window, renotify 30m. Live since 2026-09-03 12:08; immediately caught
azerothcore/storage at 95.4% and azerothcore/config at 81%.

Template renders empty labels (`Usage: 95.4%% on /`) for series lacking
ns/vol labels — cosmetic only.

### Blind spot

The ClickHouse data PVC reports to **no** SigNoz host-level source: not
`k8s.volume.*`, not `k8s.pod.filesystem` (that's the pod's root fs, not
mounts), and `system.filesystem.usage` on the node stayed flat through the
incident. The `clickhouse_sql` rule above is the only reliable signal.

Never trust `{{$value}}` on a nodata alert — check `system.disks` directly.

## Prevention

SigNoz instruments its own ClickHouse queries with OTel spans → ~5.5k
queries/min → `trace_log` grew ~8 GiB/day. With the 1d TTL cap it
stabilizes at ~8 GiB, which fits on 50 GiB but leaves little headroom for
growth.

If the volume ever refills, set `opentelemetry_start_trace_probability=0`
in ClickHouse config via the HelmRelease `clickhouse` section.

**Open:** this setting is not currently applied — it's a mitigation, not a fix.
The real fix is more headroom (larger PVC) or moving system-table load off
the main disk.

## Escalation

If the runbook doesn't resolve it:
- **Volume keeps refilling after TTL fixes** → resize PVC (requires storage
  class with `allowVolumeExpansion: true`) or increase TTL aggressiveness.
- **CH pod won't start** → check `kubectl logs` for config errors; the STS
  may have stale metadata.
- **Collector is wedged** (40+ min of queued batches, OvercommitTracker
  kills while flushing) → delete the collector pod. There is no pod-delete
  in the k8s MCP (SA is get/list/watch by design) — run it yourself.

---

## Incident log

### 2026-09-03 ~01:00 first incident

50 GiB CHI PVC (`chi-backend-clickhouse-cluster-0-0-0`, gpu2) at 100%.
Root cause: not telemetry retention — `signoz_*` DBs were ~4 GiB with all
3d TTLs already applied via setTTL API. ClickHouse's own system tables ate
the volume: `trace_log` 24.4 GiB, `query_log` 10.5 GiB, `part_log` 4.7 GiB.

Fix: dropped `trace_log` + `query_log` → 38 GiB free. Re-applied TTLs.
Fresh data landed at 11:16Z across all three signals; both alerts inactive.

### 2026-09-03 ~11:05 second incident (TTL purge storm)

`system.metric_log` TTL purge (30d→2d set ~11:05) got stuck merging 10.9k
parts → OOM retry loop → CH saturated → nodata false alarm. Fixed by
dropping `metric_log` and re-applying TTL.

### 2026-09-03 12:08 alerts went live

PVC almost full rule immediately caught a real one: azerothcore/storage
95.4% (1.4 GiB free of 30 GiB); azerothcore/config 81% warning.
