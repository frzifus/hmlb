# Gateway API Configuration

This directory contains Gateway API resources that provide modern ingress capabilities with integrated DNS management and automatic TLS certificate provisioning.

## Components

### Gateway Configuration
- **[gateway.yaml](./gateway.yaml)** - Main Gateway resource defining listeners and routing rules
- **[external-dns.yaml](./external-dns.yaml)** - Automatic DNS record management with Cloudflare

### TLS Certificate Management  
- **[lets-encrypt.yaml](./lets-encrypt.yaml)** - Let's Encrypt ClusterIssuer for automatic certificate provisioning
- **[lets-encrypt-secret.yaml](./lets-encrypt-secret.yaml)** - ACME account credentials (SOPS encrypted)
- **[dns-cloudflare-secret.yaml](./dns-cloudflare-secret.yaml)** - Cloudflare API credentials for DNS-01 challenges (SOPS encrypted)

## Features

### Automatic DNS Management
External DNS automatically creates and manages DNS records in Cloudflare based on Gateway and HTTPRoute resources, eliminating manual DNS configuration.

### TLS Certificate Automation
Cert-manager with Let's Encrypt provides:
- Automatic certificate provisioning for new domains
- DNS-01 challenge support for wildcard certificates
- Automatic certificate renewal
- Integration with Gateway API TLS configuration

### Modern Routing
Gateway API provides:
- Advanced traffic routing capabilities
- Built-in load balancing
- Header-based routing
- Request/response transformation
- Traffic splitting for canary deployments

## Configuration

The Gateway is configured to:
- Listen on ports 80 (HTTP) and 443 (HTTPS)
- Automatically redirect HTTP to HTTPS
- Support multiple hostnames with individual TLS certificates
- Integrate with MetalLB for LoadBalancer service type

## Usage

Applications can expose services through Gateway API by creating HTTPRoute resources that reference the main Gateway. The system automatically handles:
- DNS record creation in Cloudflare
- TLS certificate provisioning
- Traffic routing to backend services

Example HTTPRoute:
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-app
spec:
  parentRefs:
  - name: main-gateway
  hostnames:
  - "myapp.example.com"
  rules:
  - backendRefs:
    - name: my-app-service
      port: 80
```