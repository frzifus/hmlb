# Frzifus Homelab

A GitOps-managed Kubernetes homelab built on Talos Linux, featuring specialized nodes for storage, GPU compute, and application workloads.

## Overview

This repository contains the complete infrastructure-as-code configuration for a multi-node Kubernetes cluster running various applications including media servers, AI/ML platforms, game servers, and development tools.

## Architecture Stack

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                    APPLICATIONS                             │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │   Jellyfin   │  │     LLM      │  │ AzerothCore  │  │    Atuin     │     │
│  │ Media Stack  │  │   Platform   │  │   WoW        │  │ Shell Sync   │     │
│  │              │  │              │  │   Server     │  │              │     │
│  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────┐ │ ... │
│  │ │Jellyfin  │ │  │ │OpenWebUI │ │  │ │Auth/World│ │  │ │PostgreSQL│ │     │
│  │ │Jellyseerr│ │  │ │LlamaStack│ │  │ │Servers   │ │  │ │Server    │ │     │
│  │ │Radarr    │ │  │ │vLLM      │ │  │ │MySQL DB  │ │  │ │          │ │     │
│  │ │Sonarr    │ │  │ │ComfyUi   │ │  │ │PHPMyAdmin│ │  │ │          │ │     │
│  │ │          │ │  │ │Sigstore  │ │  │ │          │ │  │ │          │ │     │
│  │ └──────────┘ │  │ └──────────┘ │  │ └──────────┘ │  │ └──────────┘ │     │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘     │
├─────────────────────────────────────────────────────────────────────────────┤
│                                   CORE SERVICES                             │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │   Gateway    │  │ Observability│  │   Storage    │  │   Security   │     │
│  │              │  │              │  │              │  │              │     │
│  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────┐ │     │
│  │ │Envoy     │ │  │ │SigNoz    │ │  │ │OpenEBS   │ │  │ │Cert-Mgr  │ │     │
│  │ │Gateway   │ │  │ │Clickhouse│ │  │ │Mayastor  │ │  │ │Tailscale │ │     │
│  │ │Nginx     │ │  │ │Kepler    │ │  │ │Cache     │ │  │ │SOPS      │ │     │
│  │ │Ingress   │ │  │ │OTEL      │ │  │ │Replicated│ │  │ │Age       │ │     │
│  │ └──────────┘ │  │ └──────────┘ │  │ └──────────┘ │  │ └──────────┘ │     │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘     │
│                                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │  Networking  │  │     GPU      │  │   MetalLB    │  │   KubeVirt   │     │
│  │              │  │              │  │              │  │              │     │
│  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────┐ │     │
│  │ │Cilium    │ │  │ │AMD GPU   │ │  │ │L2/BGP    │ │  │ │VM        │ │     │
│  │ │CNI       │ │  │ │Plugin    │ │  │ │LoadBal.  │ │  │ │Platform  │ │     │
│  │ │No Proxy  │ │  │ │Intel GPU │ │  │ │Address   │ │  │ │CDI       │ │     │
│  │ │Mesh      │ │  │ │Plugin    │ │  │ │Pool      │ │  │ │          │ │     │
│  │ └──────────┘ │  │ └──────────┘ │  │ └──────────┘ │  │ └──────────┘ │     │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘     │
├─────────────────────────────────────────────────────────────────────────────┤
│                               KUBERNETES LAYER                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                        ┌────────────────────────────┐                       │
│                        │        Flux CD             │                       │
│                        │        GitOps              │                       │
│                        │   ┌────────────────────┐   │                       │
│                        │   │  Git Repository    │   │                       │
│                        │   │  SOPS Encryption   │   │                       │
│                        │   │  Kustomization     │   │                       │
│                        │   │  Auto Sync         │   │                       │
│                        │   └────────────────────┘   │                       │
│                        └────────────────────────────┘                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                NODE TOPOLOGY                                │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │Control Plane│  │  Storage    │  │   Worker    │  │     GPU     │         │
│  │   Nodes     │  │   Nodes     │  │   Nodes     │  │   Nodes     │         │
│  │             │  │             │  │             │  │             │         │
│  │ ┌─────────┐ │  │ ┌─────────┐ │  │ ┌─────────┐ │  │ ┌─────────┐ │         │
│  │ │master1  │ │  │ │storage1 │ │  │ │worker1  │ │  │ │gpu1     │ │         │
│  │ │master2  │ │  │ │storage2 │ │  │ │worker2  │ │  │ │gpu2     │ │         │
│  │ │master3  │ │  │ │         │ │  │ │         │ │  │ │         │ │         │
│  │ │         │ │  │ │Bonded   │ │  │ │General  │ │  │ │AMD/Intel│ │         │
│  │ │Mixed HW │ │  │ │Network  │ │  │ │Workload │ │  │ │GPU      │ │         │
│  │ │Schedul. │ │  │ │Hugepage │ │  │ │         │ │  │ │ML/AI    │ │         │
│  │ └─────────┘ │  │ └─────────┘ │  │ └─────────┘ │  │ └─────────┘ │         │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                TALOS LINUX                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│           ┌─────────────────────────────────────────────────────┐           │
│           │               Security Features                     │           │
│           │  • LUKS2 Disk Encryption                            │           │
│           │  • Secure Boot Support                              │           │
│           │  • Immutable OS                                     │           │
│           │  • API-driven Configuration                         │           │
│           │  • No SSH/Shell Access                              │           │
│           └─────────────────────────────────────────────────────┘           │
├─────────────────────────────────────────────────────────────────────────────┤
│                              PHYSICAL HARDWARE                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Architecture

- **Operating System**: Talos Linux with secure boot support
- **Kubernetes Distribution**: Vanilla Kubernetes managed by Talos
- **GitOps**: Flux CD for continuous deployment
- **Networking**: Cilium CNI with Tailscale mesh networking
- **Storage**: OpenEBS with Mayastor for high-performance storage
- **Observability**: SigNoz for logs/metrics/tracing, Kepler for power monitoring

### Cluster Topology

The cluster consists of specialized node types:

- **3 Control Plane Nodes** (master1-3): Mixed hardware for control plane workloads
- **2 Storage Nodes** (storage1-2): Dedicated storage with bonded networking and hugepages
- **2 Worker Nodes** (worker1-2): General application workloads
- **2 GPU Nodes** (gpu1-2): ML/AI compute with AMD and Intel GPU support

## Quick Start

### Prerequisites

- Talos Linux nodes configured and running
- `kubectl` configured for cluster access
- `sops-key.txt` file for secret decryption

### Bootstrap Cluster

```bash
# Bootstrap Flux CD and initial configuration
make bootstrap-k8s-homelab
```

This command sets up Flux CD and creates the necessary secrets for GitOps operations.

## Repository Structure

### Infrastructure (`infra/`)

Contains the core Talos configuration:

- `talconfig.yaml`: Complete cluster and node definitions
- `talenv.yaml`: Environment variables for Talos configuration  
- `talsecret.sops.yaml`: Encrypted secrets (disk encryption, Tailscale auth)

### Cluster Configuration (`clusters/`)

GitOps configuration organized by environment:

#### Base Configuration (`clusters/base/`)
- Flux CD installation and Git repository setup
- Shared Kustomization bases

#### Homelab Environment (`clusters/homelab/`)

**Bootstrap** (`bootstrap/`): Initial Flux setup and bootstrapping

**Core Infrastructure** (`core/`): [→ View Core Components Documentation](clusters/homelab/core/README.md)
- Networking: Cilium, MetalLB, Envoy Gateway, Nginx Ingress
- Storage: OpenEBS, persistent volume configurations  
- Security: Cert-manager, Tailscale
- Observability: SigNoz, OpenTelemetry, Kepler
- GPU support: Device plugins and Node Feature Discovery

**Applications** (`apps/`): [→ View Applications Documentation](clusters/homelab/apps/README.md)

### Container Images (`images/`)

Custom container builds:

- `acore/`: AzerothCore image for WOTLK World of Warcraft server
- `desktop/`: Custom Fedora desktop [bootc](https://github.com/bootc-dev/bootc) image with development tools
- `ganesha-nfs/`: NFS Ganesha server image

## Key Applications

### AI/ML Platform
[Enterprise-grade ML infrastructure →](clusters/homelab/apps/llm/README.md)
- [vLLM deployment with IBM Granite models →](clusters/homelab/apps/llm/vllm/README.md)
- Sigstore model validation and integrity verification
- OpenWebUI for LLM interaction
- Llama Stack for AI agent development

### Development Tools
- [Atuin shell history sync →](clusters/homelab/apps/atuin/README.md)
- Nextcloud for file sharing and collaboration
- Testing namespace for experimental deployments

### Media Server Stack
[Complete media automation and streaming setup →](clusters/homelab/apps/jellyfin/README.md)
- Jellyfin media server with hardware transcoding
- Jellyseerr for media requests
- Sonarr/Radarr for content management
- [Steam game cache and game streaming server →](clusters/homelab/apps/steam/README.md)


### Game Servers
- [AzerothCore World of Warcraft private server →](clusters/homelab/apps/azerothcore/README.md)
- [Enshrouded dedicated server →](clusters/homelab/apps/enshrouded/README.md)

## Security Features

- **Disk Encryption**: LUKS2 encryption for all system and ephemeral storage
- **Secret Management**: SOPS with age encryption for GitOps secrets
- **Network Security**: Tailscale mesh networking for secure external access
- **Model Integrity**: Sigstore-based verification for ML models
- **Secure Boot**: Support for UEFI secure boot on Talos nodes

## Development Workflow

### Making Changes

1. Modify configurations in the appropriate `clusters/homelab/` directory
2. Commit and push changes to the main branch
3. Flux CD automatically applies changes to the cluster
4. Monitor reconciliation with `kubectl get kustomizations -A`

### Adding New Applications

1. Create a new directory under `clusters/homelab/apps/`
2. Add Kubernetes manifests with appropriate namespace, storage, and networking
3. Include ServiceMonitor for observability if applicable
4. Update `clusters/homelab/apps/kustomization.yaml` to include the new app

### Custom Images

Build and push custom images from the `images/` directory. Each subdirectory contains a `Containerfile` and any necessary build context.

## Maintenance

### Dependency Updates

Renovate automatically creates pull requests for:
- Kubernetes manifest updates
- Helm chart version bumps
- Container image updates
- Flux CD component updates

### Monitoring

- **Cluster Health**: SigNoz dashboards for infrastructure metrics
- **Application Logs**: Centralized logging through observability stack
- **Power Consumption**: Kepler for energy monitoring
- **Storage Performance**: OpenEBS metrics and alerts

## Documentation Structure

This repository includes comprehensive documentation for all components:

### Core Infrastructure Documentation
- **[Core Components Overview →](clusters/homelab/core/README.md)** - Complete infrastructure component documentation
- **[Gateway API Configuration →](clusters/homelab/core/gateway/README.md)** - Modern ingress with automatic DNS and TLS
- **[GPU Support Infrastructure →](clusters/homelab/core/gpu/README.md)** - AMD and Intel GPU device plugins
- **[Storage Infrastructure →](clusters/homelab/core/storage/README.md)** - OpenEBS with Mayastor high-performance storage
- **[Observability Stack →](clusters/homelab/core/observability/README.md)** - SigNoz monitoring and tracing platform
- **[Tailscale Networking →](clusters/homelab/core/tailscale/README.md)** - Secure mesh networking and zero-trust access
- **[MetalLB Load Balancer →](clusters/homelab/core/metallb/README.md)** - Bare-metal load balancing
- **[KubeVirt Virtualization →](clusters/homelab/core/kubevirt/README.md)** - Virtual machine platform

### Application Documentation
- **[Applications Overview →](clusters/homelab/apps/README.md)** - Complete application portfolio
- **[Jellyfin Media Stack →](clusters/homelab/apps/jellyfin/README.md)** - Media server with automation
- **[LLM Platform →](clusters/homelab/apps/llm/README.md)** - AI/ML inference platform
- **[AzerothCore WoW Server →](clusters/homelab/apps/azerothcore/README.md)** - World of Warcraft private server
- **[Enshrouded Game Server →](clusters/homelab/apps/enshrouded/README.md)** - Survival game dedicated server
- **[Steam Game Cache- and Streaming Platform →](clusters/homelab/apps/steam/README.md)** - Steam cache- and streaming server

