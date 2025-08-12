# GPU Support Infrastructure

This directory contains the configuration for GPU support in the Kubernetes cluster, enabling ML/AI workloads on both AMD and Intel GPU hardware.

## Components

### Device Plugins
- **[amd-device-plugin.yaml](./amd-device-plugin.yaml)** - AMD GPU device plugin for resource allocation
- **[intel-gpu-plugin.yaml](./intel-gpu-plugin.yaml)** - Intel GPU device plugin for resource allocation
- **[amd-device-plugin-nfd.yaml](./amd-device-plugin-nfd.yaml)** - AMD device plugin with Node Feature Discovery integration

### Hardware Discovery
- **[node-feature-discovery.yaml](./node-feature-discovery.yaml)** - Automatic hardware feature detection and labeling
- **[gpu-node-feature-rules.yaml](./gpu-node-feature-rules.yaml)** - Custom rules for GPU-specific node labeling

## Features

### Multi-Vendor GPU Support
The cluster supports both AMD and Intel GPUs with dedicated device plugins that:
- Expose GPU resources to the Kubernetes scheduler
- Handle resource allocation and isolation
- Provide GPU-specific node labels for workload placement

### Automatic Hardware Detection
Node Feature Discovery automatically:
- Detects available GPU hardware
- Labels nodes with GPU capabilities
- Enables automatic scheduling of GPU workloads
- Provides hardware-specific feature labels

### Resource Management
GPU resources are managed through:
- Custom resource types (e.g., `amd.com/gpu`, `gpu.intel.com/i915`)
- Node selectors for GPU-enabled workloads
- Resource limits and requests for fair sharing

## Supported Hardware

### AMD GPUs
- Consumer and professional AMD graphics cards
- ROCm runtime support for ML frameworks
- Multiple GPU per node support

### Intel GPUs
- Intel Arc and Iris Xe graphics
- Intel GPU plugin for media processing and AI acceleration
- Integrated graphics support

## Usage

### Requesting GPU Resources

Workloads can request GPU resources in their pod specifications:

**AMD GPU:**
```yaml
resources:
  limits:
    amd.com/gpu: 1
  requests:
    amd.com/gpu: 1
```

**Intel GPU:**
```yaml
resources:
  limits:
    gpu.intel.com/i915: 1
  requests:
    gpu.intel.com/i915: 1
```

### Node Affinity

Use node selectors to target specific GPU types:
```yaml
nodeSelector:
  feature.node.kubernetes.io/pci-0300_1002.present: "true"  # AMD GPU
  # or
  feature.node.kubernetes.io/pci-0300_8086.present: "true"  # Intel GPU
```

## Monitoring

GPU usage and health are monitored through:
- Node exporter GPU metrics
- Device plugin health checks
- SigNoz dashboards for GPU utilization
- Kepler power consumption monitoring for GPU workloads

## Troubleshooting

Common issues and solutions:
- Verify GPU drivers are installed on nodes
- Check device plugin pod logs for initialization errors
- Ensure NFD has labeled nodes correctly
- Confirm GPU resources are available in node status