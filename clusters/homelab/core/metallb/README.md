# MetalLB Load Balancer

This directory contains the MetalLB configuration for providing LoadBalancer services in the bare-metal Kubernetes environment.

## Components

- **[metallb.yaml](./metallb.yaml)** - MetalLB operator and controller deployment
- **[config.yaml](./config.yaml)** - IP address pools and BGP configuration
- **[network.yaml](./network.yaml)** - Network policies for MetalLB traffic

## Features

### Load Balancer Services
MetalLB enables LoadBalancer service types in bare-metal environments by:
- **IP Address Assignment**: Automatic external IP allocation from configured pools
- **Traffic Distribution**: Layer 2 or BGP-based load balancing
- **Service Integration**: Seamless integration with Kubernetes Service resources
- **High Availability**: Automatic failover for service endpoints

### Address Pool Management
Configured IP address pools for different service types:
- **Public Services**: Externally accessible services
- **Private Services**: Internal network access only
- **Reserved Ranges**: Dedicated IP ranges for specific applications
- **Dynamic Allocation**: Automatic IP assignment from available pools

## Configuration Modes

### Layer 2 Mode
Default configuration using Layer 2 for:
- **Simple Setup**: No router configuration required
- **ARP/NDP**: Address resolution at Layer 2
- **Single Node**: One node announces IP at a time
- **Fast Failover**: Quick recovery during node failures

### BGP Mode (Optional)
Advanced BGP configuration for:
- **Multi-path Routing**: ECMP load distribution
- **Router Integration**: Direct integration with network infrastructure
- **Scalable Design**: Better performance for high-traffic services
- **Graceful Shutdown**: Controlled traffic drain during maintenance

## Service Types

### Ingress Controllers
MetalLB provides external IPs for:
- **Nginx Ingress**: HTTP/HTTPS traffic handling
- **Envoy Gateway**: Modern API gateway services
- **Custom Ingress**: Application-specific ingress controllers

### Application Services
Direct external access for:
- **Database Services**: External database connectivity
- **Monitoring Tools**: Observability platform access
- **Development Services**: Remote development environments
- **Gaming Services**: Game server external access

## Network Integration

### Cilium Integration
MetalLB works with Cilium CNI for:
- **Traffic Routing**: Efficient packet forwarding
- **Network Policies**: Security policy enforcement
- **Load Balancing**: Distributed load balancing
- **Observability**: Network flow monitoring

### External Access
Services are accessible through:
- **Direct IP Access**: External IP assignment
- **DNS Integration**: Automatic DNS record creation
- **Port Management**: Flexible port allocation
- **Protocol Support**: TCP, UDP, and SCTP protocols

## Address Pool Configuration

### Pool Allocation
IP addresses are allocated from pools based on:
- **Service Annotations**: Explicit pool selection
- **Namespace Labels**: Automatic pool assignment
- **Service Type**: Default pools for different service categories
- **Resource Constraints**: Available IP address limits

### IP Range Management
- **CIDR Notation**: Flexible IP range specification
- **Auto-assignment**: Dynamic IP allocation
- **Reserved Addresses**: Static IP assignment for critical services
- **Pool Sharing**: Multiple services sharing pool resources

## Monitoring and Observability

### Metrics Collection
MetalLB exposes metrics for:
- **IP Pool Usage**: Available vs. allocated addresses
- **Service Status**: LoadBalancer service health
- **BGP Sessions**: BGP peer connectivity (if enabled)
- **Traffic Statistics**: Load balancer traffic patterns

### Health Monitoring
- **Controller Health**: MetalLB controller pod status
- **Speaker Health**: Node-level speaker pod monitoring
- **Service Reachability**: External connectivity testing
- **Failover Events**: Load balancer failover tracking

## Troubleshooting

### Common Issues
- **IP Exhaustion**: Pool capacity and allocation monitoring
- **ARP Conflicts**: Layer 2 address resolution problems
- **BGP Connectivity**: Routing protocol troubleshooting
- **Service Annotation**: Proper service configuration

### Debugging Commands
```bash
# Check MetalLB controller status
kubectl get pods -n metallb-system

# View IP pool configuration  
kubectl get ipaddresspools -n metallb-system

# Check service external IP assignment
kubectl get svc -A -o wide

# View MetalLB logs
kubectl logs -n metallb-system -l app=metallb
```

## Security Considerations

### Network Policies
MetalLB traffic is controlled through:
- **Speaker Policies**: Control plane communication rules
- **BGP Policies**: Routing protocol security (if enabled)
- **Service Policies**: Application-specific network rules
- **Ingress Filtering**: Traffic source validation

### Access Control
- **RBAC Configuration**: Controller service account permissions
- **Network Segmentation**: Isolated MetalLB traffic flows
- **Audit Logging**: Load balancer configuration changes
- **Certificate Management**: BGP authentication (if enabled)