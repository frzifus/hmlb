# Jellyfin Media Server Stack

A complete media automation and streaming platform featuring Jellyfin media server with hardware transcoding and automated content management through Sonarr/Radarr.

## Components

### Media Server
- **[jellyfin.yaml](./jellyfin.yaml)** - Jellyfin media server with GPU hardware transcoding
- **[jellyseerr.yaml](./jellyseerr.yaml)** - Media request and discovery platform

### Content Management
- **[sonarr.yaml](./sonarr.yaml)** - TV series management and automation
- **[radarr.yaml](./radarr.yaml)** - Movie management and automation

### Storage and Infrastructure
- **[pvc.yaml](./pvc.yaml)** - Persistent volume claims for media storage
- **[transcoding-cache.yaml](./transcoding-cache.yaml)** - Fast storage for transcoding cache
- **[credentials.yaml](./credentials.yaml)** - Service credentials (SOPS encrypted)
- **[vpn-credentials.yaml](./vpn-credentials.yaml)** - VPN authentication (SOPS encrypted)

### Observability
- **[otel-auto-instr.yaml](./otel-auto-instr.yaml)** - OpenTelemetry automatic instrumentation

## Features

### Hardware-Accelerated Transcoding
Jellyfin is configured with GPU hardware acceleration for:
- **Real-time Transcoding**: Multiple concurrent streams with minimal CPU usage
- **Format Support**: Wide range of video/audio codecs and containers
- **Quality Adaptation**: Automatic quality adjustment based on client capabilities
- **Tone Mapping**: HDR to SDR conversion for broader compatibility

### Automated Content Management
Complete automation workflow:
1. **Content Discovery**: Jellyseerr provides user-friendly media requests
2. **Automatic Search**: Sonarr/Radarr search for requested content
3. **Quality Profiles**: Configurable quality and format preferences
5. **Organization**: Automatic file organization and metadata fetching
6. **Library Updates**: Jellyfin automatically detects new content

### Secure traffic
VPN includes integration for:
- **Privacy Protection**: All traffic routed through VPN
- **Kill Switch**: Stop operation if VPN connection fails
- **IP Leak Protection**: DNS and IPv6 leak prevention

## Storage Architecture

### Media Storage
- **Storage Class**: `openebs-media` for large file storage with good throughput
- **Capacity**: Optimized for storing large video files
- **Replication**: Configurable replication for data protection
- **Performance**: Balanced for streaming workloads

### Transcoding Cache
- **Storage Class**: `openebs-cache` for ultra-fast temporary storage
- **Purpose**: Temporary storage for transcoded video segments
- **Performance**: High IOPS SSD storage for real-time transcoding
- **Cleanup**: Automatic cleanup of old transcoded files

### Configuration Storage
- **Persistent Volumes**: Application configurations and databases
- **Backup Strategy**: Configuration backed up through persistent storage
- **State Management**: Stateful application data persistence

## Networking and Access

### External Access
- **Tailscale Integration**: Secure remote access to media services
- **Jellyfin Web UI**: Browser-based media streaming interface
- **Mobile Apps**: Native mobile app support through secure tunnels
- **Client Applications**: Support for various media player clients

### Service Communication
- **Internal Networking**: Services communicate via Kubernetes DNS
- **Port Configuration**: Standard ports for each service component
- **Load Balancing**: Automatic load balancing for high availability
- **Network Policies**: Controlled communication between components

## GPU Integration

### Hardware Requirements
- **AMD GPU**: AMD graphics cards with VAAPI support
- **Intel GPU**: Intel integrated graphics with Quick Sync
- **Driver Support**: Proper GPU drivers installed on worker nodes
- **Device Plugin**: Kubernetes GPU device plugin for resource allocation

### Transcoding Configuration
- **Hardware Acceleration**: GPU-accelerated encoding/decoding
- **Codec Support**: H.264, H.265, VP9, and AV1 support (hardware dependent)
- **Quality Settings**: Configurable quality presets for different scenarios
- **Resource Limits**: GPU resource limits to prevent resource exhaustion

## Monitoring and Observability

### Application Metrics
- **Jellyfin Metrics**: Server performance, transcoding load, user sessions
- **Storage Metrics**: Disk usage, I/O performance, capacity planning
- **Network Metrics**: Bandwidth usage, streaming quality, connection counts

### OpenTelemetry Integration
- **Distributed Tracing**: Request tracing across media stack components
- **Performance Monitoring**: Application performance insights
- **Error Tracking**: Automatic error detection and alerting
- **Custom Metrics**: Media-specific business metrics

### Health Monitoring
- **Service Health**: Kubernetes health checks for all components
- **GPU Health**: GPU utilization and temperature monitoring
- **VPN Health**: VPN connection status and IP verification
- **Storage Health**: Disk health and capacity alerting

## Security Considerations

### Network Security
- **Network Policies**: Restricted communication between components
- **Firewall Rules**: Controlled external access through Tailscale
- **SSL/TLS**: Encrypted communication for all web interfaces

### Access Control
- **User Authentication**: Jellyfin user management and permissions
- **API Security**: Secure API access for automation tools
- **Credential Management**: Encrypted storage of service credentials
- **Audit Logging**: Access and activity logging for security monitoring

### Data Protection
- **Backup Strategy**: Regular backups of configuration and metadata
- **Data Encryption**: Encrypted storage for sensitive configuration
- **Recovery Procedures**: Documented disaster recovery processes
- **Update Management**: Controlled updates with rollback capabilities

## Configuration Management

### Service Configuration
Each service includes customizable configuration for:
- **Quality Profiles**: Video/audio quality preferences

### Integration Settings
- **API Keys**: Secure API communication between services
- **Webhook Configuration**: Real-time notifications and triggers
- **Import/Export**: Configuration backup and migration tools
- **Custom Scripts**: Post-processing and automation scripts

## Troubleshooting

### Common Issues
- **Transcoding Problems**: GPU driver and hardware acceleration issues
- **Storage Issues**: Disk space and performance problems
- **Network Connectivity**: Service discovery and communication issues

### Debugging Tools
- **Log Analysis**: Centralized logging through SigNoz
- **Performance Profiling**: Resource usage and bottleneck identification
- **Network Diagnostics**: Connection testing and traffic analysis
- **GPU Monitoring**: Hardware utilization and error detection

### Maintenance Procedures
- **Regular Updates**: Service updates and security patches
- **Database Maintenance**: Periodic database cleanup and optimization
- **Storage Cleanup**: Automated cleanup of temporary and old files
- **Configuration Backup**: Regular backup of service configurations
