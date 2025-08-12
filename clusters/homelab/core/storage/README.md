# Storage Infrastructure

This directory contains the storage configuration for the Kubernetes cluster, built on OpenEBS with Mayastor for high-performance container-attached storage.

## Components

### OpenEBS Configuration
- **[openebs.yaml](./openebs.yaml)** - OpenEBS operator and Mayastor engine deployment
- **[disks.yaml](./disks.yaml)** - Disk pool configuration for storage nodes

### Storage Classes
- **[cache.yaml](./cache.yaml)** - Fast SSD storage class for caching and temporary data
- **[crucial.yaml](./crucial.yaml)** - High-performance NVMe storage class for databases
- **[media.yaml](./media.yaml)** - Large capacity storage class for media files

## Architecture

### Storage Nodes
The cluster includes two dedicated storage nodes (storage1, storage2) with:
- **Bonded Networking**: Redundant network connections for high availability
- **Hugepages**: Optimized memory allocation for Mayastor performance
- **Direct-attached Storage**: NVMe SSDs for low-latency data access
- **NUMA Awareness**: CPU and memory locality optimization

### Mayastor Engine
Mayastor provides:
- **NVMe-over-TCP**: High-performance storage protocol
- **Replication**: Synchronous data replication across nodes
- **Thin Provisioning**: Efficient storage space utilization
- **Snapshot Support**: Point-in-time data recovery

## Storage Classes

### Cache Storage (`mayastor-cache`)
- **Use Case**: Redis, temporary data, build caches
- **Performance**: Ultra-low latency SSD storage
- **Replication**: Single replica for maximum performance
- **Capacity**: Optimized for small, frequent I/O operations

### Crucial Storage (`mayastor-crucial`) 
- **Use Case**: Databases, critical application data
- **Performance**: High IOPS NVMe storage
- **Replication**: Multi-replica for data safety
- **Consistency**: Synchronous replication across storage nodes

### Media Storage (`mayastor-media`)
- **Use Case**: Media files, backups, large datasets
- **Performance**: Balanced throughput and capacity
- **Replication**: Configurable based on data importance
- **Capacity**: Large volume support for bulk storage

## Usage

### Requesting Storage

Applications can request storage using persistent volume claims:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: database-storage
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: openebs-crucial
  resources:
    requests:
      storage: 10Gi
```

### Storage Class Selection

Choose storage classes based on workload requirements:
- **Database workloads**: `openebs-crucial` for consistency and performance
- **Cache layers**: `openebs-cache` for maximum speed
- **Media storage**: `openebs-media` for capacity and cost efficiency

## Monitoring

Storage infrastructure is monitored through:
- **OpenEBS Metrics**: Pool utilization, IOPS, latency
- **Node Metrics**: Disk usage, network throughput
- **SigNoz Dashboards**: Storage performance visualization
- **Alerting**: Automatic notifications for storage issues

## Backup and Recovery

### Snapshots
Mayastor supports volume snapshots for:
- Point-in-time recovery
- Data migration between environments
- Testing with production data copies

### Replication
Data is automatically replicated across storage nodes to ensure:
- High availability during node maintenance
- Protection against hardware failures
- Zero-downtime storage operations

## Performance Tuning

### Node Configuration
Storage nodes are optimized with:
- Hugepages allocation for Mayastor
- CPU isolation for storage workloads
- Network bonding for redundancy
- NUMA-aware resource allocation

### I/O Optimization
- NVMe drives configured for maximum performance
- Queue depth optimization for workload patterns
- Latency-sensitive workload prioritization
- Background compaction scheduling
