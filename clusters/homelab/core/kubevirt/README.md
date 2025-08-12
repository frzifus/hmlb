# KubeVirt Virtualization Platform

This directory contains the KubeVirt configuration for running virtual machines alongside containers in the Kubernetes cluster.

## Components

- **[kubevirt-operator.yaml](./kubevirt-operator.yaml)** - KubeVirt operator for lifecycle management
- **[kubevirt-cr.yaml](./kubevirt-cr.yaml)** - KubeVirt custom resource configuration
- **[namespace.yaml](./namespace.yaml)** - Dedicated namespace for KubeVirt components

## Features

### Virtual Machine Management
KubeVirt provides native Kubernetes support for:
- **VM Lifecycle**: Start, stop, pause, and migrate virtual machines
- **Live Migration**: Zero-downtime VM migration between nodes
- **Snapshots**: Point-in-time VM state capture and restore
- **Templates**: Reusable VM configuration templates

### Container-VM Integration
Seamless integration between VMs and containers:
- **Shared Networking**: VMs participate in Kubernetes networking
- **Storage Integration**: VMs use Kubernetes persistent volumes
- **Service Discovery**: VMs accessible through Kubernetes services
- **Resource Management**: Unified resource allocation and limits

### Enterprise Features
Production-ready virtualization capabilities:
- **High Availability**: VM restart and migration policies
- **Resource Scheduling**: NUMA-aware VM placement
- **Device Assignment**: GPU and hardware passthrough
- **Security Isolation**: VM workload isolation and security

## Virtual Machine Types

### Legacy Application Support
Run traditional applications that require:
- **Full OS**: Complete operating system environments
- **Kernel Modules**: Custom kernel module loading
- **Hardware Access**: Direct hardware device access
- **Legacy Dependencies**: Applications not suitable for containerization

### Development Environments
Isolated development environments for:
- **Multi-OS Testing**: Windows, Linux, and other OS testing
- **Network Simulation**: Complex network topology testing
- **Security Testing**: Isolated security research environments
- **Legacy System Integration**: Testing with legacy systems

### Specialized Workloads
Virtual machines for specialized use cases:
- **Database Systems**: Traditional database deployments
- **Network Appliances**: Virtual network function deployment
- **Desktop Environments**: Virtual desktop infrastructure (VDI)
- **Compliance Workloads**: Regulatory compliance requirements

## Resource Management

### CPU and Memory
VMs are allocated resources through:
- **CPU Requests/Limits**: Guaranteed and maximum CPU allocation
- **Memory Requests/Limits**: Memory allocation and over-commit policies
- **NUMA Topology**: Optimal memory and CPU locality
- **CPU Pinning**: Dedicated CPU core assignment for performance

### Storage Integration
VMs use Kubernetes storage through:
- **PVC Integration**: Persistent volume claims for VM disks
- **Storage Classes**: Different storage performance tiers
- **Live Storage Migration**: Moving VM storage without downtime
- **Snapshot Support**: VM disk snapshots for backup and cloning

### Networking
VM networking leverages Kubernetes networking:
- **Pod Networking**: VMs get pod IP addresses
- **Service Integration**: VMs accessible through Kubernetes services
- **Network Policies**: Traffic control and segmentation
- **Multi-tenancy**: Isolated network namespaces for VMs

## High Availability

### VM Migration
Automatic and manual VM migration for:
- **Node Maintenance**: Drain nodes for updates
- **Resource Rebalancing**: Optimize cluster resource utilization
- **Failure Recovery**: Automatic restart on healthy nodes
- **Performance Optimization**: Move VMs to less loaded nodes

### Backup and Recovery
VM backup and disaster recovery through:
- **VM Snapshots**: Application-consistent point-in-time backups
- **Export/Import**: VM migration between clusters
- **Template Management**: Golden image management
- **Disaster Recovery**: Cross-cluster VM replication

## GPU and Hardware Acceleration

### GPU Passthrough
Virtual machines can access GPU hardware:
- **AMD GPU**: AMD graphics card passthrough
- **Intel GPU**: Intel graphics integration
- **NVIDIA GPU**: NVIDIA GPU virtualization (if available)
- **Custom Devices**: PCI device passthrough for specialized hardware

### Performance Optimization
VMs are optimized for performance through:
- **Hugepages**: Large page memory allocation
- **CPU Affinity**: NUMA-aware CPU scheduling
- **SR-IOV**: Hardware-accelerated networking
- **Device Emulation**: Optimized virtual hardware

## Monitoring and Observability

### VM Metrics
KubeVirt exposes metrics for:
- **Resource Usage**: CPU, memory, disk, and network utilization
- **VM Status**: Running, migrating, stopped states
- **Performance**: VM performance characteristics
- **Migration Events**: VM migration success and failure rates

### Integration with Cluster Observability
- **SigNoz Integration**: VM metrics in cluster observability
- **Alerting**: VM-specific alert rules and notifications
- **Log Collection**: VM guest OS log aggregation
- **Distributed Tracing**: VM service tracing integration

## Security

### VM Isolation
Security boundaries between VMs and containers:
- **Hypervisor Isolation**: Hardware-assisted virtualization
- **Network Segmentation**: VM network isolation
- **Storage Isolation**: Separate VM storage domains
- **Resource Limits**: Prevent resource exhaustion attacks

### Access Control
VM access control through:
- **RBAC Integration**: Kubernetes role-based access control
- **VM Permissions**: Fine-grained VM operation permissions
- **Console Access**: Secure VM console access
- **SSH Integration**: Key-based VM access management

## Getting Started

### Creating a VM
Example VM configuration:
```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: example-vm
spec:
  running: true
  template:
    spec:
      domain:
        devices:
          disks:
          - name: harddrive
            disk:
              bus: virtio
        machine:
          type: "q35"
        resources:
          requests:
            memory: 1Gi
            cpu: 1
      volumes:
      - name: harddrive
        persistentVolumeClaim:
          claimName: example-vm-disk
```

### VM Management Commands
```bash
# List virtual machines
kubectl get vms

# Start a VM
virtctl start example-vm

# Connect to VM console
virtctl console example-vm

# Migrate a VM
virtctl migrate example-vm
```