# LLM Application Stack

A comprehensive Large Language Model (LLM) deployment featuring vLLM inference, OpenWebUI frontend, and Llama Stack for AI applications. This setup includes model validation using Sigstore for enhanced security and integrity verification.

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   OpenWebUI     │────│   Llama Stack   │────│      vLLM       │
│   (Frontend)    │    │   (Orchestrator)│    │   (Inference)   │
│   Port: 3000    │    │   Port: 8321    │    │   Port: 8000    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       │
    llm.klimlive.de         Tavily Search            IBM Granite
                              API Key                3.3-2B Model
                                                  + Sigstore Validation
```

## Components

### 1. vLLM Inference Engine
- **Model**: Llama 3.2-70B Instruct
- **Features**: 
  - High-performance LLM inference
  - Sigstore-based model integrity validation
  - OpenTelemetry instrumentation
  - Automatic model validation on deployment
- **Storage**: Persistent volume for model cache
- **Security**: Model validation using Sigstore transparency logs

### 2. OpenWebUI
- **Purpose**: Web-based chat interface for LLM interaction
- **Features**:
  - Modern chat UI supporting multiple conversations
  - Integration with Llama Stack API
  - OpenTelemetry tracing enabled
  - Persistent data storage
- **Access**: Available at `llm.klimlive.de`

### 3. Llama Stack
- **Purpose**: Orchestration layer and API gateway
- **Features**:
  - OpenAI-compatible API endpoints
  - Tavily Search API integration for web search capabilities
  - Python instrumentation for observability
  - Configurable via YAML templates

## Model Security & Validation

This deployment implements Sigstore-based model validation for enhanced security:

### Automatic Validation
- Models are automatically validated on pod startup
- Validation is triggered by the `validation.rhtas.redhat.com/ml: "true"` label
- Init containers verify model integrity before the main workload starts

### Manual Validation
```bash
# Restart deployment to trigger validation
kubectl rollout restart deployment vllm -n llm

# Check validation status
kubectl get pods -n llm
kubectl logs <vllm-pod-name> -c model-validation -n llm
```

### Debug Container
A debug container is available for manual model operations:
```bash
# Sign a model
kubectl exec -it <debug-pod> -- model_signing sign sigstore /models/...

# Verify a model
kubectl exec -it <debug-pod> -- model_signing verify sigstore /models/...

# View debug container
kubectl get pod -l app=model-validation-debug -n llm
```

## Configuration

### Environment Variables
- **vLLM**: 
  - `HF_TOKEN`: Hugging Face access token (from secret)
  - Model validation settings in granite-validation.yaml
- **Llama Stack**:
  - `VLLM_URL`: Connection to vLLM service
  - `TAVILY_SEARCH_API_KEY`: Web search integration
  - `CUSTOM_OTEL_TRACE_ENDPOINT`: Observability endpoint
- **OpenWebUI**:
  - `OPENAI_API_BASE_URL`: Points to Llama Stack API
  - `ENABLE_OTEL`: OpenTelemetry integration

### Storage
- **OpenWebUI**: 3Gi persistent volume (openebs-cache)
- **vLLM**: 100Gi persistent volume for model storage (openebs-cache)
- **Llama Stack**: EmptyDir volumes for temporary storage

## Networking

- **External Access**: HTTPRoute via Envoy Gateway at `llm.klimlive.de`
- **Internal Communication**: 
  - OpenWebUI → Llama Stack (port 8321)
  - Llama Stack → vLLM (port 8000)
- **Load Balancing**: ClusterIP services for internal traffic

## Observability

All components are instrumented with OpenTelemetry:
- **Traces**: Sent to SigNoz backend in observability namespace
- **Metrics**: ServiceMonitor for Prometheus scraping (vLLM)
- **Health Checks**: Kubernetes readiness and liveness probes

## Deployment

The application is deployed via Flux CD GitOps:

```bash
# Check deployment status
kubectl get pods -n llm
kubectl get svc -n llm
kubectl get httproute -n llm

# View logs
kubectl logs -f deployment/vllm -n llm
kubectl logs -f deployment/open-webui -n llm
kubectl logs -f deployment/llamastack -n llm
```

## Security Features

1. **Model Integrity**: Sigstore validation ensures model authenticity
2. **Secret Management**: SOPS-encrypted secrets for API keys
3. **Network Security**: Internal service communication only
4. **Resource Limits**: CPU and memory constraints on all components
5. **Minimal Privileges**: Non-root containers where possible

## Troubleshooting

### Common Issues
1. **Model validation failures**: Check granite-validation.yaml configuration
2. **vLLM startup issues**: Verify GPU availability and model download
3. **OpenWebUI connection errors**: Check Llama Stack service connectivity
4. **Search functionality**: Verify Tavily API key configuration

### Debug Commands
```bash
# Check model validation operator
kubectl get modelvalidation -n llm

# View vLLM model loading
kubectl logs deployment/vllm -n llm

# Test API connectivity
kubectl exec -it <openwebui-pod> -- curl http://llamastack:8321/health
```

## Resource Requirements

- **vLLM**: Requires GPU nodes for optimal performance
- **Total CPU**: ~3 cores
- **Total Memory**: ~10Gi
- **Storage**: ~103Gi total persistent storage
- **GPU**: AMD/Intel GPU support via device plugins

## Related Documentation

- [vLLM Component README](vllm/README.md) - Detailed vLLM configuration and model validation
- [Sigstore Model Transparency](https://github.com/sigstore/model-transparency/) - Model validation details
