# Use the shared OpenWebUI credential unless a CLI-specific credential is set.
if [ -n "${OPENWEBUI_API_KEY:-}" ]; then
    if [ -z "${OLLAMA_API_KEY:-}" ]; then
        export OLLAMA_API_KEY="$OPENWEBUI_API_KEY"
    fi
    if [ -z "${ANTHROPIC_AUTH_TOKEN:-}" ]; then
        export ANTHROPIC_AUTH_TOKEN="$OPENWEBUI_API_KEY"
    fi
fi
