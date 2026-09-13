# Keep zplug's read-only code separate from per-user writable state.
export ZPLUG_ROOT="/usr/lib/zplug"
export ZPLUG_HOME="${XDG_CONFIG_HOME:-$HOME/.config}/zsh-zplug"
export ZPLUG_BIN="${ZPLUG_HOME}/bin"
export ZPLUG_CACHE_DIR="${ZPLUG_HOME}/cache"
export ZPLUG_REPOS="${ZPLUG_HOME}/repos"
export ZPLUG_LOADFILE="${ZPLUG_HOME}/packages.zsh"

export PATH="${HOME}/bin:${HOME}/.local/bin:${PATH}:${ZPLUG_ROOT}/bin:${ZPLUG_HOME}/bin:${PATH}"

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

# xterm-color no longer exists in Fedora terminfo (gone by F44); tput/infocmp
# fail *silently* with it (broke the zplug install build step). Only default
# TERM when unset: buildah RUN steps have none, desktop terminal emulators
# (foot, xterm-256color, ...) set their own and must not be overridden.
[[ -n "$TERM" ]] || export TERM=xterm-256color
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