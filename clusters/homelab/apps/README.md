# Applications

This directory contains all production application deployments for the homelab cluster. Applications are organized by category and include complete Kubernetes manifests with networking, storage, monitoring, and security configurations.

## Application Categories

### AI/ML Platform
Enterprise-grade machine learning infrastructure:
- **[LLM Stack](./llm/)** - Large language model deployment with vLLM, OpenWebUI, and Llama Stack for AI agent development

### Development and Productivity
Tools for development and personal productivity:
- **[Atuin](./atuin/)** - Shell history synchronization across devices
- **[Nextcloud](./nextcloud/)** - File sharing and collaboration platform

### Media Server Stack
Complete media automation and streaming platform:
- **[Jellyfin Media Server](./jellyfin/)** - Media streaming with hardware transcoding, content management
- **[Steam](./steam/)** - Steam game server hosting platform

### Game Servers
Dedicated game server hosting:
- **[AzerothCore](./azerothcore/)** - World of Warcraft private server with custom configurations
- **[Enshrouded](./enshrouded/)** - Dedicated survival game server

### External Services
Proxy and tunnel services for external applications:
- **[External FEWO](./ext-fewo/)** - External vacation rental application proxy
- **[External KlimLive](./ext-klimlive/)** - External climate monitoring service proxy

### Utility Services
Supporting services and utilities:
- **[Ark Overseer](./ark-overseer/)** - ARK game server management
- **[Quixsi](./quixsi/)** - Custom utility service
- **[Redirect Services](./redirect/)** - HTTP redirect services for various endpoints
- **[Testing](./testing/)** - Experimental and testing deployments

## Common Patterns

### Storage Configuration
Applications use OpenEBS storage classes based on requirements:
- **Database workloads**: `openebs-crucial` for consistency and performance
- **Media storage**: `openebs-media` for large file storage
- **Cache/temporary data**: `openebs-cache` for high-speed access

### Networking and Access
All applications include:
- **Tailscale Integration**: Secure external access through VPN mesh
- **Service Monitoring**: Prometheus metrics collection via ServiceMonitor
- **Network Policies**: Traffic segmentation and security policies
- **Ingress Configuration**: HTTP/HTTPS access through Gateway API or Nginx

### Security and Secrets
- **SOPS Encryption**: All secrets encrypted in Git using age encryption
- **RBAC**: Appropriate service account permissions and role bindings
- **Network Segmentation**: Isolated namespaces with controlled communication
- **TLS Certificates**: Automatic certificate management via cert-manager

### Observability
Applications include comprehensive monitoring:
- **Metrics Collection**: Custom and standard metrics via Prometheus
- **Distributed Tracing**: OpenTelemetry integration for request tracing
- **Log Aggregation**: Centralized logging through SigNoz
- **Health Checks**: Kubernetes liveness and readiness probes

## Deployment Management

### GitOps Workflow
Applications are deployed through Flux CD:
1. Configuration changes committed to Git
2. Flux automatically detects and applies changes
3. Applications reconcile to desired state
4. Monitoring alerts on deployment issues

### Resource Management
- **Resource Limits**: All containers have appropriate CPU/memory limits
- **Node Affinity**: Applications scheduled on appropriate node types
- **Priority Classes**: Critical applications get scheduling priority
- **Disruption Budgets**: Controlled rolling updates and maintenance

### Backup and Recovery
- **Persistent Data**: Important data stored on replicated storage
- **Configuration Backup**: All configuration in Git for disaster recovery
- **Secrets Backup**: Encrypted secrets recoverable from Git repository
- **State Management**: Stateful applications designed for cluster migration

## Adding New Applications

### Directory Structure
New applications should follow the standard structure:
```
app-name/
├── namespace.yaml          # Dedicated namespace
├── deployment.yaml         # Main application deployment
├── service.yaml           # Kubernetes service
├── pvc.yaml              # Persistent volume claims
├── secret.yaml           # SOPS-encrypted secrets
├── servicemonitor.yaml   # Prometheus monitoring
├── network.yaml          # Network policies
└── kustomization.yaml    # Kustomize configuration
```

### Integration Requirements
New applications must include:
- Dedicated namespace with appropriate labels
- Resource requests and limits
- Health check endpoints
- Monitoring and observability integration
- Security policies and network isolation
- Backup strategy for persistent data

### Review Process
1. Create application manifests following established patterns
2. Test in the testing namespace first
3. Add ServiceMonitor for observability
4. Update main kustomization.yaml to include new app
5. Monitor deployment and verify functionality

## Performance and Scaling

### Resource Optimization
- Applications are sized based on actual usage patterns
- GPU workloads scheduled on dedicated GPU nodes
- Storage-intensive workloads use high-performance storage classes
- Network-intensive applications consider node network capacity

### High Availability
- Critical applications deployed with multiple replicas
- Pod disruption budgets prevent simultaneous updates
- Anti-affinity rules distribute replicas across nodes
- Health checks enable automatic pod replacement

### Scaling Strategies
- **Horizontal Pod Autoscaler**: Automatic scaling based on metrics
- **Vertical Pod Autoscaler**: Right-sizing resource requests
- **Cluster Autoscaler**: Node scaling for capacity requirements
- **Manual Scaling**: Predictable workload scaling procedures
