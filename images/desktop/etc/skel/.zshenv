export PATH="${HOME}/bin:${HOME}/.local/bin:${PATH}:${HOME}/.zplug/bin:${PATH}"

export GOROOT=/usr/lib/golang
export GOPATH=${HOME}/git/golang_workspace
export PATH="$GOROOT/bin:$PATH"
export PATH="$PATH:$GOPATH/bin"
export GOPROXY='https://proxy.golang.org,direct'

export LC_ALL=de_DE.UTF-8
export LANG=de_de.UTF-8

export HISTFILE=~/.zsh_history
export HISTSIZE=20000000
export HISTFILESIZE=200000
export SAVEHIST=$HISTSIZE

export LC_ALL=de_DE.UTF-8
export LANG=de_DE.UTF-8

export TERM=xterm-color
source "$HOME/.cargo/env"
. "$HOME/.cargo/env"

export SSH_AUTH_SOCK="${XDG_RUNTIME_DIR}/keyring/ssh"

export PATH=~/.npm-global/bin:$PATH

export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_METRICS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
export OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp.klimlive.de

export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer CHANGE_ME"
export OLLAMA_API_KEY="CHANGE_ME"