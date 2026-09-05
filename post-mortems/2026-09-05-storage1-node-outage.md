# Storage1 node outage — 2026-09-05

Status: draft · Author: frzifus · Written: 2026-09-05 07:51 UTC
Impact window: 2026-09-05 01:45 UTC → ongoing
Related: SigNoz alert "Node Availability Alert" (critical, firing since 02:05 UTC)

> **Before reboot (time-critical):** the Talos log ring buffer does not
> survive reboot. Capture storage1 logs immediately:
>
> ```
> talosctl -n storage1 dmesg > storage1-dmesg.log
> talosctl -n storage1 logs kubelet > storage1-kubelet.log
> talosctl -n storage1 logs machined > storage1-machined.log
> ```
>
> Grep for: `nvme`, `hung_task`, `soft lockup`, `watchdog`, `link up/down`.

## Summary

storage1, the 4-core mayastor storage node, hung silently and was marked
NotReady at 01:45 UTC after ~25 minutes of host-level CPU and network
saturation. The trigger was an unthrottled mayastor rebuild-style data flood:
at 01:20 UTC the node's 2x1G LACP bond went from a ~35 MB/s baseline to a
sustained ~230 MB/s (line rate, TX-heavy), which drove network-softirq CPU
load to 7–11.5 on 4 cores. Kubelet starved, stopped posting heartbeats, and
the node went Ready=Unknown. There was no OOM, no kernel panic logged, and no
disk I/O stress — the node slowed to death rather than crashing loudly.

Worst affected: every openebs-crucial (DB) volume replica — the crucial
storage class has repl=2 but only one DiskPool, which lives on storage1.

## Timeline (all UTC, 2026-09-05)

| Time | Event |
|---|---|
| ≤ Sep 4 | Foreshadowing: io-engine NVMe/TCP errors ("controller in failed state", "Failed to read the CC register"), repeated rebuild cancellations on storage2; storage1 pods show chronic readiness-probe timeouts for 24h+ |
| 01:20:00 | storage1 host network jumps 6x to ~230 MB/s sustained (TX ~135 / RX ~95). storage2 (+60 MB/s) and master2 (+83 MB/s) jump at the same instant — pattern of a nexus on master2 rebuilding a replica, pulling from storage1 |
| 01:20–01:44 | Node CPU 2.7 → 4.0 of 4 cores (idle ≈ 0), load 7 → 11.5. Disk telemetry stays quiet throughout (no queue growth, no io_time spike). io-engine itself flat at ~1.9–2.0 cores — the extra ~1 core is kernel network-stack (softirq) work |
| 01:44:23 | Last log line from storage1 (benign vnc chatter, metallb BGP reconcile). All metrics stop |
| 01:45:22 | Kubelet stops posting status. Node → Ready=Unknown, `node.kubernetes.io/unreachable` NoSchedule/NoExecute taints applied |
| 02:05:46 | SigNoz "Node Availability Alert" begins firing (6 of 7 hosts reporting) |
| ~02:15 | 13 workloads rescheduled by their controllers to other nodes and recover Running (mcp-k8s-server, both postgres-collectors, observer, fladder replicas, homepage, phpmyadmin, steam-frame-watcher, …) |
| ongoing | 14 pods stuck Terminating on the dead node; 4 StatefulSets cannot recreate their pods because controllers wait on the stuck Terminating pods: garmin/frzifus-exporter-0, ext-mastodon database-1, ext-oxicloud database-1, immich database-3 |

## Root cause

Unthrottled mayastor rebuild traffic saturated the storage1 uplink, and the
resulting network softirq on a 4-core node starved kubelet until it stopped
posting heartbeats. Contributing factors, in order of weight:

1. **No uplink headroom.** 2x1G LACP bond ≈ 230 MB/s ceiling was fully
   consumed by the flood for 24 minutes.
2. **No CPU reservation for system services.** kubelet allocatable is
   3950m/4000m — zero reserved. SPDK busy-polling io-engine is pinned to
   cores 1,2 (half the node), leaving no budget for softirq spikes.
3. **No watchdog / panic-on-hang.** A saturated node has no path to
   self-recovery; it stays wedged until a human reboots it (5+ hours and
   counting).
4. **Unclear replica spread in openebs-crucial.** The git manifest suggests
   repl=2 with only storage1-crucial (storage2-crucial is commented out);
   in practice storage2-crucial exists (Online, 1.8 TiB) and the manifest is stale. Verify crucial volumes actually have replicas on both pools.
5. **Version drift on storage1 io-engine.** The mayastor HelmRelease has
   been stuck in a failed upgrade since June 21 (context deadline exceeded);
   storage1 io-engine runs v2.10.0 while storage2 runs v2.11.1 (OnDelete
   strategy), so fixes in 2.11 never reached storage1.
6. **No rebuild throttle exists in mayastor 2.10/2.11.** Verified against
   io-engine source (env.rs): no REBUILD_* env vars; only gRPC
   RebuildJob pause()/resume() per job.
7. **No dedicated storage nodes.** storage1 also runs kubevirt VMs
   (virt-launcher) and misc pods; storage nodes share the general pool.

## What went well

- Detection worked: SigNoz node alert fired within ~20 min of node death.
- Self-healing: all StatefulSet/Deployment workloads that could move did
  move and recovered without manual intervention.
- Forensics were possible entirely from SigNoz despite no kernel logs:
  network rates, CPU load, disk telemetry, and per-pod CPU localized the
  cause precisely.

## What went poorly / gaps

- No host-level (journald/kernel/Talos) log ingestion in SigNoz — only
  /var/log/pods container logs. The final kernel-level wedge mechanism
  (NIC reset vs SPDK lockup) cannot be confirmed from telemetry.
- Detection-to-response gap: the SigNoz alert fired within 20 min of node
  death (02:05 UTC), but a human did not engage for ~5 hours. No paging
  integration — alerts post to a webhook channel that is not actively
  monitored outside working hours.
- 4 StatefulSets blocked for hours on stuck Terminating pods
  (no force-delete automation/policy).

## Action items

| # | Action | Owner | Status |
|---|---|---|---|
| 1 | Reconcile `crucial.yaml` git manifest with live cluster state: `storage2-crucial` DiskPool already exists (Online, 1.8 TiB) but is commented out in the repo. Uncomment, fix `node: storage1` → `node: storage2`, and verify crucial volumes spread across both pools | frzifus | open |
| 2 | Apply Talos hardening on storage1+storage2: `watchdogTimeout: 10m`, `kernel.panic: "10"`, `kernel.panic_on_oops: "1"`, kubeReserved cpu 500m / mem 512Mi / ephemeral 1Gi | frzifus | [frzifus/hmlb#833](https://github.com/frzifus/hmlb/pull/833) |
| 3 | SigNoz alerts (created 2026-09-05, channel LocalWebhook, renotify 15m): storage-node CPU load % of allocatable cores (warn >80% 10m, crit >100%); storage-node TX rate (warn >30 MB/s 10m, crit >60 MB/s; baseline 0.9–3 MB/s) | frzifus | done (ruleIds 01a07072-9500-70e3-a4f5-68abf50fb9fe, 01a07072-…b254 — see SigNoz) |
| 4 | Re-run the stalled mayastor HelmRelease reconciliation; get storage1 io-engine to v2.11.1 (OnDelete: delete pod after node returns) | frzifus | open |
| 5 | Capture storage1 logs **before reboot** (see callout above); analyze after node returns | frzifus | open |
| 6 | Force-delete the 4 blocked StatefulSet pods (or wait for node return). **Caveat:** verify the volume is no longer attached on storage1 (or cordon/isolate the node) before force-deleting — risk of split-brain / data corruption if the old node still holds the volume | frzifus | open |
| 7 | Longer term: more cores per storage node or split serving/client and replica roles; consider serialize-rebuild runbook (cordon nodes one at a time after outages) | frzifus | open |

## Evidence

- SigNoz: `system.network.io` (host, direction=transmit/receive) — 6x jump at 01:20, sustained 24 min; per-pod network flat → hostNetwork io-engine traffic.
- SigNoz: `system.cpu.load_average.1m` 7 → 11.46; `k8s.node.cpu.usage` 2.7 → 4.0 cores; `k8s.pod.cpu.usage` io-engine flat ~1.9–2.0 cores, all other pods negligible.
- SigNoz: `system.disk.*` (io_time, pending_operations, weighted_io_time) quiet through the entire window → not disk I/O.
- k8s.node.memory.working_set flat ~7.3/12.9 GiB → not OOM.
- Node condition history: Ready=Unknown, "Kubelet stopped posting node status", unreachable taint at 01:45:22.
- Last container logs on storage1 end 01:44:23; no error/FATAL lines before death.
