# Enable Powerlevel10k instant prompt. Should stay close to the top of ~/.zshrc.
# Initialization code that may require console input (password prompts, [y/n]
# confirmations, etc.) must go above this block; everything else may go below.
if [[ -r "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh" ]]; then
  source "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh"
fi

# zsh plugins
ZPLUG_HOME=${HOME}/.zplug
source $ZPLUG_HOME/init.zsh
#zplug 'zplug/zplug', hook-build:'zplug --self-manage'
#zplug "tobjaw/fd6ff79d37678e5b66c7455c75b710f7", from:gist, as:theme
zplug romkatv/powerlevel10k, as:theme, depth:1
#zplug "plugins/colored-man-pages", from:oh-my-zsh
zplug "plugins/history", from:oh-my-zsh
zplug "plugins/docker", from:oh-my-zsh
zplug "chrissicool/zsh-256color"
zplug "zsh-users/zsh-syntax-highlighting"
zplug "zsh-users/zsh-autosuggestions"
zplug load

bindkey -e
# https://linux.die.net/man/1/zshzle
bindkey '^P' up-history
bindkey '^N' down-history
bindkey '^?' backward-delete-char
bindkey '^h' backward-delete-char
bindkey "^a" beginning-of-line
bindkey "^e" end-of-line
bindkey "^R" history-incremental-pattern-search-backward
alias eo="emacsclient -c -n"
alias ec="emacsclient -n"
alias ls="eza"
alias tf="terraform"
alias ll="eza -abghHlS"
alias open=xdg-open
alias k=kubectl
alias kind="systemd-run --scope --user kind"

export do="--dry-run=client -o yaml"

# fzf
export FZF_DEFAULT_COMMAND="fd --hidden --strip-cwd-prefix --exclude .git"
export FZT_CTRL_T_COMMAND="$FZF_DEFAULT_COMMAND"
export FZF_ALT_C_COMMAND="fd --type=d --hidden --strip-cwd-prefix --exclude .git"

alias ff='fzf -m --preview "bat --style=numbers --color=always {}"'

## case-insensitive (uppercase from lowercase) completion
# zstyle ':completion:*' matcher-list 'm:{a-z}={A-Z}'
## case-insensitive (all) completion
#zstyle ':completion:*' matcher-list 'm:{a-zA-Z}={A-Za-z}'
## case-insensitive,partial-word and then substring completion
zstyle ':completion:*' matcher-list 'm:{a-z}={A-Z}' 'r:|[._-]=* r:|=*' 'l:|=* r:|=*'


# alias monitor=xrandr --output HDMI2 --auto; sleep 2; xrandr --output eDP1 --off
# alias laptop=xrandr --output eDP1 --auto; sleep 2; xrandr --output HDMI2 --off

#zplug "plugins/docker-compose", from:oh-my-zsh
#zplug "plugins/docker", from:oh-my-zsh

export LESS_TERMCAP_mb=$'\e[1;32m'
export LESS_TERMCAP_md=$'\e[1;32m'
export LESS_TERMCAP_me=$'\e[0m'
export LESS_TERMCAP_se=$'\e[0m'
export LESS_TERMCAP_so=$'\e[01;33m'
export LESS_TERMCAP_ue=$'\e[0m'
export LESS_TERMCAP_us=$'\e[1;4;31m'

autoload -U +X bashcompinit && bashcompinit

if (( $+commands[kubectl] )); then
  source <(kubectl completion zsh)
  complete -F __start_kubectl k
fi
# To customize prompt, run `p10k configure` or edit ~/.p10k.zsh.
[[ ! -f ~/.p10k.zsh ]] || source ~/.p10k.zsh


# atuin

# shellcheck disable=SC2034,SC2153,SC2086,SC2155

# Above line is because shellcheck doesn't support zsh, per
# https://github.com/koalaman/shellcheck/wiki/SC1071, and the ignore: param in
# ludeeus/action-shellcheck only supports _directories_, not _files_. So
# instead, we manually add any error the shellcheck step finds in the file to
# the above line ...

# Source this in your ~/.zshrc
autoload -U add-zsh-hook

zmodload zsh/datetime 2>/dev/null

# If zsh-autosuggestions is installed, configure it to use Atuin's search. If
# you'd like to override this, then add your config after the $(atuin init zsh)
# in your .zshrc
# _zsh_autosuggest_strategy_atuin() {
#     suggestion=$(ATUIN_QUERY="$1" atuin search --cmd-only --limit 1 --search-mode prefix)
# }

# if [ -n "${ZSH_AUTOSUGGEST_STRATEGY:-}" ]; then
#     ZSH_AUTOSUGGEST_STRATEGY=("atuin" "${ZSH_AUTOSUGGEST_STRATEGY[@]}")
# else
#     ZSH_AUTOSUGGEST_STRATEGY=("atuin")
# fi

export ATUIN_SESSION=$(atuin uuid)
ATUIN_HISTORY_ID=""

_atuin_preexec() {
    local id
    id=$(atuin history start -- "$1")
    export ATUIN_HISTORY_ID="$id"
    __atuin_preexec_time=${EPOCHREALTIME-}
}

_atuin_precmd() {
    local EXIT="$?" __atuin_precmd_time=${EPOCHREALTIME-}

    [[ -z "${ATUIN_HISTORY_ID:-}" ]] && return

    local duration=""
    if [[ -n $__atuin_preexec_time && -n $__atuin_precmd_time ]]; then
        printf -v duration %.0f $(((__atuin_precmd_time - __atuin_preexec_time) * 1000000000))
    fi

    (ATUIN_LOG=error atuin history end --exit $EXIT ${duration:+--duration=$duration} -- $ATUIN_HISTORY_ID &) >/dev/null 2>&1
    export ATUIN_HISTORY_ID=""
}

_atuin_search() {
    emulate -L zsh
    zle -I

    # swap stderr and stdout, so that the tui stuff works
    # TODO: not this
    local output
    # shellcheck disable=SC2048
    output=$(ATUIN_SHELL_ZSH=t ATUIN_LOG=error ATUIN_QUERY=$BUFFER atuin search $* -i 3>&1 1>&2 2>&3)

    zle reset-prompt

    if [[ -n $output ]]; then
        RBUFFER=""
        LBUFFER=$output

        if [[ $LBUFFER == __atuin_accept__:* ]]
        then
            LBUFFER=${LBUFFER#__atuin_accept__:}
            zle accept-line
        fi
    fi
}
_atuin_search_vicmd() {
    _atuin_search --keymap-mode=vim-normal
}
_atuin_search_viins() {
    _atuin_search --keymap-mode=vim-insert
}

_atuin_up_search() {
    # Only trigger if the buffer is a single line
    if [[ ! $BUFFER == *$'\n'* ]]; then
        _atuin_search --shell-up-key-binding "$@"
    else
        zle up-line
    fi
}
_atuin_up_search_vicmd() {
    _atuin_up_search --keymap-mode=vim-normal
}
_atuin_up_search_viins() {
    _atuin_up_search --keymap-mode=vim-insert
}

add-zsh-hook preexec _atuin_preexec
add-zsh-hook precmd _atuin_precmd

zle -N atuin-search _atuin_search
zle -N atuin-search-vicmd _atuin_search_vicmd
zle -N atuin-search-viins _atuin_search_viins
zle -N atuin-up-search _atuin_up_search
zle -N atuin-up-search-vicmd _atuin_up_search_vicmd
zle -N atuin-up-search-viins _atuin_up_search_viins

# These are compatibility widget names for "atuin <= 17.2.1" users.
zle -N _atuin_search_widget _atuin_search
zle -N _atuin_up_search_widget _atuin_up_search

bindkey -M emacs '^r' atuin-search
bindkey -M viins '^r' atuin-search-viins
bindkey -M vicmd '/' atuin-search
# bindkey -M emacs '^[[A' atuin-up-search
# bindkey -M vicmd '^[[A' atuin-up-search-vicmd
# bindkey -M viins '^[[A' atuin-up-search-viins
# bindkey -M emacs '^[OA' atuin-up-search
# bindkey -M vicmd '^[OA' atuin-up-search-vicmd
# bindkey -M viins '^[OA' atuin-up-search-viins
# bindkey -M vicmd 'k' atuin-up-search-vicmd
