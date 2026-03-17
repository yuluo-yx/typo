# typo - Command auto-correction for zsh
#
# Installation:
#   Add to ~/.zshrc:
#     source /path/to/typo/install/typo.zsh
#
#   Or use:
#     eval "$(typo init zsh)"
#
# Usage:
#   1. Type a wrong command, press 'ff' to fix before executing
#   2. After executing a failed command, press 'ff' to fix last command
#
# Example:
#   $ gut stattus<ff>  →  git status
#   $ gut stattus      →  command not found
#   $ <ff>             →  git status

_typo_fix_command() {
    local cmd="${BUFFER}"
    local stderr_file="${TYPO_STDERR_CACHE:-/tmp/typo-stderr-$$}"

    # If buffer is empty, get last command from history
    if [[ -z "$cmd" ]]; then
        cmd=$(fc -ln -1 | sed 's/^[[:space:]]*//')
    fi

    [[ -z "$cmd" ]] && return

    local fixed
    if [[ -f "$stderr_file" && -s "$stderr_file" ]]; then
        fixed=$(typo fix -s "$stderr_file" "$cmd" 2>/dev/null)
    else
        fixed=$(typo fix "$cmd" 2>/dev/null)
    fi

    if [[ -n "$fixed" && "$fixed" != "$cmd" ]]; then
        BUFFER="$fixed"
        CURSOR=${#BUFFER}
        zle reset-prompt
    fi
}

zle -N _typo_fix_command

# ff to fix command
bindkey 'ff' _typo_fix_command

# stderr capture hook
_typo_preexec() {
    TYPO_STDERR_CACHE="/tmp/typo-stderr-$$"
    exec 2> >(tee "$TYPO_STDERR_CACHE" >&2)
}

autoload -Uz add-zsh-hook
add-zsh-hook preexec _typo_preexec
