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
#   1. Type a wrong command, press <Esc><Esc> to fix before executing
#   2. After executing a failed command, press <Esc><Esc> to fix last command
#
# Example:
#   $ gut stattus<Esc><Esc>  →  git status
#   $ gut stattus      →  command not found
#   $ <Esc><Esc>       →  git status

_typo_fix_command() {
    local cmd="${BUFFER}"
    local stderr_file="${TYPO_STDERR_CACHE:-}"
    local fixed=""
    local last_exit_code="${TYPO_LAST_EXIT_CODE:-0}"
    local use_last_command=0

    # If buffer is empty, get last command from history
    if [[ -z "$cmd" ]]; then
        use_last_command=1
        cmd=$(fc -ln -1 | sed 's/^[[:space:]]*//')
    fi

    [[ -z "$cmd" ]] && return

    if [[ "$use_last_command" -eq 1 && -f "$stderr_file" && -s "$stderr_file" ]]; then
        fixed=$(typo fix --exit-code "$last_exit_code" -s "$stderr_file" "$cmd" 2>/dev/null)
    elif [[ "$use_last_command" -eq 1 ]]; then
        fixed=$(typo fix --exit-code "$last_exit_code" "$cmd" 2>/dev/null)
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

# Esc+Esc to fix command
bindkey '\e\e' _typo_fix_command

_typo_current_shell_id() {
    print -r -- "$$"
}

_typo_owns_stderr_cache() {
    [[ -n "${TYPO_STDERR_CACHE:-}" && "${TYPO_STDERR_CACHE_OWNER:-}" == "$(_typo_current_shell_id)" ]]
}

_typo_owns_original_stderr_fd() {
    [[ -n "${TYPO_ORIG_STDERR_FD:-}" && "${TYPO_ORIG_STDERR_FD_OWNER:-}" == "$(_typo_current_shell_id)" ]]
}

# Initialize the stderr cache file for the current shell session.
_typo_init_stderr_cache() {
    local tmp_dir="${TMPDIR:-/tmp}"
    local shell_id
    local stderr_cache=""

    shell_id="$(_typo_current_shell_id)"

    if [[ -n "${TYPO_STDERR_CACHE:-}" && "${TYPO_STDERR_CACHE_OWNER:-}" != "$shell_id" ]]; then
        unset TYPO_STDERR_CACHE
        unset TYPO_STDERR_CACHE_OWNER
    fi

    if _typo_owns_stderr_cache && [[ -f "$TYPO_STDERR_CACHE" && -w "$TYPO_STDERR_CACHE" ]]; then
        return 0
    fi

    unset TYPO_STDERR_CACHE
    unset TYPO_STDERR_CACHE_OWNER

    if command -v mktemp >/dev/null 2>&1; then
        stderr_cache=$(mktemp "$tmp_dir/typo-stderr-XXXXXX" 2>/dev/null)
    fi
    if [[ -z "$stderr_cache" ]]; then
        stderr_cache="$tmp_dir/typo-stderr-$$"
        : >| "$stderr_cache" 2>/dev/null || return 1
    fi
    if [[ ! -f "$stderr_cache" || ! -w "$stderr_cache" ]]; then
        return 1
    fi

    TYPO_STDERR_CACHE="$stderr_cache"
    TYPO_STDERR_CACHE_OWNER="$shell_id"
}

# Remove stale caches older than one day for the current user to avoid piling up in /tmp.
_typo_cleanup_stale_caches() {
    emulate -L zsh
    setopt extended_glob null_glob

    local tmp_dir="${TMPDIR:-/tmp}"
    local file

    for file in "$tmp_dir"/typo-stderr-*(.Nm+1); do
        [[ -O "$file" ]] || continue
        [[ "$file" == "${TYPO_STDERR_CACHE:-}" ]] && continue
        rm -f -- "$file" 2>/dev/null
    done
}

# Save the original stderr so repeated commands do not keep chaining shell file descriptors.
_typo_save_original_stderr() {
    local shell_id

    shell_id="$(_typo_current_shell_id)"

    if [[ -n "${TYPO_ORIG_STDERR_FD:-}" && "${TYPO_ORIG_STDERR_FD_OWNER:-}" != "$shell_id" ]]; then
        unset TYPO_ORIG_STDERR_FD
        unset TYPO_ORIG_STDERR_FD_OWNER
    fi

    if ! _typo_owns_original_stderr_fd; then
        exec {TYPO_ORIG_STDERR_FD}>&2
        TYPO_ORIG_STDERR_FD_OWNER="$shell_id"
    fi
}

_typo_restore_stderr() {
    if _typo_owns_original_stderr_fd; then
        exec 2>&$TYPO_ORIG_STDERR_FD
    fi
}

_typo_preexec() {
    _typo_init_stderr_cache || return
    _typo_save_original_stderr
    : >| "$TYPO_STDERR_CACHE"
    exec 2> >(tee "$TYPO_STDERR_CACHE" >&$TYPO_ORIG_STDERR_FD)
}

_typo_precmd() {
    TYPO_LAST_EXIT_CODE=$?
    _typo_restore_stderr
}

_typo_zshexit() {
    _typo_restore_stderr

    if _typo_owns_original_stderr_fd; then
        exec {TYPO_ORIG_STDERR_FD}>&-
    fi
    unset TYPO_ORIG_STDERR_FD
    unset TYPO_ORIG_STDERR_FD_OWNER

    if _typo_owns_stderr_cache; then
        rm -f -- "$TYPO_STDERR_CACHE" 2>/dev/null
    fi
    unset TYPO_STDERR_CACHE
    unset TYPO_STDERR_CACHE_OWNER
}

autoload -Uz add-zsh-hook
add-zsh-hook -D preexec _typo_preexec 2>/dev/null
add-zsh-hook -D precmd _typo_precmd 2>/dev/null
add-zsh-hook -D zshexit _typo_zshexit 2>/dev/null
add-zsh-hook preexec _typo_preexec
add-zsh-hook precmd _typo_precmd
add-zsh-hook zshexit _typo_zshexit

_typo_init_stderr_cache
_typo_save_original_stderr
_typo_cleanup_stale_caches

# Mark shell integration as loaded
export TYPO_SHELL_INTEGRATION=1
