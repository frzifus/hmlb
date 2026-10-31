# inference — LiteLLM gateway

Internal AI gateway based on [LiteLLM](https://github.com/BerriAI/litellm)
(DB-less mode). Single chokepoint for model traffic inside the cluster and the
backend for OpenWebUI. Replaced the earlier praxis-ai deployment (kept the
StatefulSet/Service name `inference` so consumers and NetworkPolicies are
untouched).

```
laptop Claude ──> llm.klimlive.de (OpenWebUI /api, anthropic-compat) ──┐
OpenWebUI chat users ─────────────────────────────────────────────────┤
                                                                      ▼
                                       inference.llm.svc.cluster.local (LiteLLM)
                                       ├─ ollama/glm-5.3-flash            → ollama.com/v1
                                       ├─ ollama/deepseek-v4.1-flash:1017 → ollama.com/v1
                                       └─ vllm/<id>                       → future local vLLM

in-cluster services (hermes, cronjobs, tools) — access granted solely by
opening a NetworkPolicy to app: inference.
```

## Why a gateway

Providers change; consumer configuration should not. Every consumer asks for a
stable, provider-prefixed model id (`ollama/…`, `vllm/…`) and the gateway maps
it to whatever backend currently serves it. Swapping providers or backends is a
pure `model_list` edit in `litellm-config.yaml` — OpenWebUI and hermes never
change.

## Deployment

- `litellm.yaml` — StatefulSet `inference` (replicas 2, stateless → replicas are
  interchangeable), image-pinned `ghcr.io/berriai/litellm-non_root`, Service
  `inference` (ClusterIP, 80 → 4000). Restricted PodSecurity enforced
  (`runAsNonRoot` UID 1000, caps dropped, RuntimeDefault seccomp).
- `litellm-config.yaml` — ConfigMap with the LiteLLM config. **Access policy is
  the config**: a model id not in `model_list` does not exist at the gateway.
- `secret-ollama-cloud.yaml` — ollama.com API key (sops-encrypt).
- `secret-litellm-master-key.yaml` — the single static shared key protecting
  LiteLLM's admin surface and authorizing model calls (sops-encrypt).

No accounts, no database (DB-less mode). Consumers send the master key from
their own Secret/env; in-cluster reachability is a NetworkPolicy concern.

## Consumers

| Consumer | Access | Model ids |
| --- | --- | --- |
| OpenWebUI (external users, Claude over `/api`) | connection slot 3, key = master key | `ollama/…`, `vllm/…` |
| hermes agents | egress NetworkPolicy → `app: inference:80` | `ollama/glm-5.3-flash` |
| cronjobs (`github-*`) | suspended — re-point at OpenWebUI `/api` or here | TBD |

## Notes

- Health: `GET /health/readiness` and `GET /health/liveliness` (unauthenticated,
  used as readiness/liveness probes). Prometheus metrics on the same port once
  enabled via `--use_prometheus_metrics`.
- LiteLLM honors standard OTEL env vars for tracing export (backend-collector).
  Configured later together with the ServiceMonitor.
- History: praxis-ai (0.8.0) served this role briefly; replaced by LiteLLM
  because ① the Anthropic path already lived in OpenWebUI, ② no official
  praxis image compiles the `otel` feature, ③ no client authentication, and
  LiteLLM covers provider routing + prefixes natively.