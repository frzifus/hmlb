# CRC (OpenShift Local) on KubeVirt

## Overview

CRC runs as a KubeVirt VM with cloud-init handling bootstrap:
- **Pull secret** written to `/opt/crc/pull-secret` and `/var/lib/kubelet/config.json`
- **`CRC_SELF_SUFFICIENT=1`** set in `/etc/sysconfig/crc-env` to enable standalone mode
- **crio + kubelet** enabled on boot

### Example

```bash
╰─❯ k get pods
NAME                      READY   STATUS      RESTARTS   AGE
crc-debug                 1/1     Running     0          118m
virt-launcher-crc-m4jkx   2/2     Running     0          2m58s

╰─❯ k get vmi -o wide
NAME   AGE    PHASE     IP            NODENAME   READY   LIVE-MIGRATABLE   PAUSED
crc    3m6s   Running   10.244.8.16   gpu2       True    False

╰─❯ k top pod
NAME                      CPU(cores)   MEMORY(bytes)
crc-debug                 24m          5Mi
virt-launcher-crc-m4jkx   1874m        6296Mi

╰─❯ k exec  -it crc-debug -- bash
[root@crc-debug /]# oc get nodes
NAME   STATUS   ROLES                         AGE   VERSION
crc    Ready    control-plane,master,worker   34d   v1.34.2

[root@crc-debug /]# ssh -i ~/.ssh/id_ecdsa_crc -o StrictHostKeyChecking=no core@crc-api 
Red Hat Enterprise Linux CoreOS 9.6.20260112-0
  Part of OpenShift 4.21, RHCOS is a Kubernetes-native operating system
  managed by the Machine Config Operator (`clusteroperator/machine-config`).

Last login: Wed Mar  4 20:57:00 2026 from 10.244.8.38
[core@crc ~]$ 
```

NOTE: cannot migrate VMI: PVC crc-disk is not shared, **live migration requires that all PVCs must be shared (using ReadWriteMany access mode)**

## Common kubectl virt commands

Start / Stop vm:

```bash
kubectl virt stop crc -n openshift
kubectl virt start crc -n openshift
```

Get vnc access:
```bash
kubectl virt vnc crc -n openshift
```

Get console:
```bash
kubectl virt console crc -n openshift
```

## Debug Pod

```bash
kubectl apply -f debug-pod.yaml
kubectl exec -it crc-debug -n openshift -- bash

# SSH into VM
ssh -i ~/.ssh/id_ecdsa_crc -o StrictHostKeyChecking=no core@crc-api

# Check OpenShift status (from debug pod)
oc get nodes
oc get co
oc get pods -A
```

## Common Issues

### Services not starting (crc-pullsecret, crc-dnsmasq, etc. all dead)

`CRC_SELF_SUFFICIENT` is set to `0`. Fix:

```bash
# Inside VM
sudo sed -i 's/CRC_SELF_SUFFICIENT=0/CRC_SELF_SUFFICIENT=1/' /etc/sysconfig/crc-env
sudo systemctl restart crc-custom.target
```

### Image pull unauthorized

Pull secret missing from CRI-O config:

```bash
# Inside VM
sudo cp /opt/crc/pull-secret /var/lib/kubelet/config.json
sudo systemctl restart crio kubelet
```

### kube-controller-manager CrashLoopBackOff / `api-int.crc.testing` not resolving

CoreDNS (`10.96.0.10`) isn't running yet (chicken-and-egg with CNI). Redirect to local dnsmasq:

```bash
# Inside VM
sudo iptables -t nat -A OUTPUT -d 10.96.0.10/32 -p udp --dport 53 -j DNAT --to-destination 192.168.126.11:53
sudo iptables -t nat -A OUTPUT -d 10.96.0.10/32 -p tcp --dport 53 -j DNAT --to-destination 192.168.126.11:53
```

### Node stuck in NotReady with stale taints

```bash
# From debug pod
oc adm taint node crc node.kubernetes.io/unreachable- node.kubernetes.io/not-ready-
```

### OVN / Multus pods not scheduling

Check if kube-controller-manager is running (see above). It manages DaemonSet reconciliation.

## Disk Provisioning

To re-provision the disk (e.g. version upgrade), update `CRC_VERSION` in `job-disk-loader.yaml`:

```bash
kubectl virt stop crc -n openshift
kubectl delete job crc-disk-loader -n openshift
kubectl apply -f job-disk-loader.yaml
# Wait for completion, then start VM
kubectl virt start crc -n openshift
```
