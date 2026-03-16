# typo - Command auto-correction for zsh
# Press ESC ESC to fix the current command

# Cache directory for stderr
TYPO_STDERR_CACHE="/tmp/typo-stderr-${$}"

# Fix the current command
_typo_fix_command() {
    local cmd="${BUFFER}"
    local stderr_file="${TYPO_STDERR_CACHE}"

    # Try to fix the command
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
        return 0
    fi

    return 1
}

# Widget to trigger fix
zle -N _typo_fix_command

# Bind ESC ESC to trigger fix
bindkey '\e\e' _typo_fix_command

# Optional: Capture stderr from last command for error parsing
# Uncomment the following to enable automatic stderr capture
#
# _typo_preexec() {
#     TYPO_LAST_CMD="$1"
# }
#
# _typo_precmd() {
#     if [[ -n "$TYPO_LAST_CMD" ]]; then
#         # Capture last command's stderr would require exec redirect
#         # This is left as optional due to complexity
#     fi
# }
#
# autoload -Uz add-zsh-hook
# add-zsh-hook preexec _typo_preexec
# add-zsh-hook precmd _typo_precmd

# Learning function - learn a correction
typo-learn() {
    if [[ $# -lt 2 ]]; then
        echo "Usage: typo-learn <wrong_command> <correct_command>"
        return 1
    fi
    typo learn "$1" "$2"
}

# Show current rules
typo-rules() {
    typo rules list
}

# Show history
typo-history() {
    typo history list
}

# Clear history
typo-clear-history() {
    typo history clear
}

echo "typo initialized. Press ESC ESC to fix commands."