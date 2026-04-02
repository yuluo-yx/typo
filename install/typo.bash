# typo - Command auto-correction for bash
#
# Installation:
#   Add to ~/.bashrc:
#     source /path/to/typo/install/typo.bash
#
#   Or use:
#     eval "$(typo init bash)"
#
# Usage:
#   1. Type a wrong command, press <Esc><Esc> to fix before executing
#   2. After executing a failed command, press <Esc><Esc> to fix last command
#
# Example:
#   $ gut stattus<Esc><Esc>  ->  git status
#   $ gut stattus      ->  command not found
#   $ <Esc><Esc>       ->  git status

_typo_fix_command() {
    local cmd="${READLINE_LINE:-}"
    local stderr_file="${TYPO_STDERR_CACHE:-}"
    local fixed=""
    local last_exit_code="${TYPO_LAST_EXIT_CODE:-0}"
    local use_last_command=0

    # If buffer is empty, get last command from history.
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
        # Preview fixes for the current readline buffer should not be persisted.
        fixed=$(typo fix --no-history "$cmd" 2>/dev/null)
    fi

    if [[ -n "$fixed" && "$fixed" != "$cmd" ]]; then
        READLINE_LINE="$fixed"
        READLINE_POINT=${#READLINE_LINE}
    fi
}

_typo_current_shell_id() {
    printf '%s\n' "$$"
}

_typo_owns_stderr_cache() {
    [[ -n "${TYPO_STDERR_CACHE:-}" && "${TYPO_STDERR_CACHE_OWNER:-}" == "$(_typo_current_shell_id)" ]]
}

_typo_owns_original_stderr_fd() {
    [[ -n "${TYPO_ORIG_STDERR_FD:-}" && "${TYPO_ORIG_STDERR_FD_OWNER:-}" == "$(_typo_current_shell_id)" ]]
}

# Initialize stderr cache file for this shell session.
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
        : > "$stderr_cache" 2>/dev/null || return 1
    fi
    if [[ ! -f "$stderr_cache" || ! -w "$stderr_cache" ]]; then
        return 1
    fi

    TYPO_STDERR_CACHE="$stderr_cache"
    TYPO_STDERR_CACHE_OWNER="$shell_id"
}

# Remove stale caches older than one day.
_typo_cleanup_stale_caches() {
    local tmp_dir="${TMPDIR:-/tmp}"
    local file

    while IFS= read -r -d '' file; do
        [[ "$file" == "${TYPO_STDERR_CACHE:-}" ]] && continue
        rm -f -- "$file" 2>/dev/null
    done < <(find "$tmp_dir" -maxdepth 1 -type f -name 'typo-stderr-*' -mtime +1 -print0 2>/dev/null)
}

# Save the original stderr once to avoid chaining shell descriptors.
_typo_save_original_stderr() {
    local shell_id

    shell_id="$(_typo_current_shell_id)"

    if [[ -n "${TYPO_ORIG_STDERR_FD:-}" && "${TYPO_ORIG_STDERR_FD_OWNER:-}" != "$shell_id" ]]; then
        unset TYPO_ORIG_STDERR_FD
        unset TYPO_ORIG_STDERR_FD_OWNER
    fi

    if ! _typo_owns_original_stderr_fd; then
        exec 3>&2
        TYPO_ORIG_STDERR_FD=3
        TYPO_ORIG_STDERR_FD_OWNER="$shell_id"
    fi
}

_typo_restore_stderr() {
    if _typo_owns_original_stderr_fd; then
        exec 2>&3
    fi
}

_typo_preexec() {
    _typo_init_stderr_cache || return
    _typo_save_original_stderr
    : > "$TYPO_STDERR_CACHE"
    exec 2> >(tee "$TYPO_STDERR_CACHE" >&3)
}

# Called from PROMPT_COMMAND; capture previous command status and restore stderr.
_typo_precmd() {
    local status=$?
    TYPO_LAST_EXIT_CODE=$status
    _typo_restore_stderr
    TYPO_READY_FOR_PREEXEC=1
}

_typo_bashexit() {
    _typo_restore_stderr

    if _typo_owns_original_stderr_fd; then
        exec 3>&-
    fi
    unset TYPO_ORIG_STDERR_FD
    unset TYPO_ORIG_STDERR_FD_OWNER

    if _typo_owns_stderr_cache; then
        rm -f -- "$TYPO_STDERR_CACHE" 2>/dev/null
    fi
    unset TYPO_STDERR_CACHE
    unset TYPO_STDERR_CACHE_OWNER
}

_typo_debug_trap() {
    # Guard against recursive DEBUG trap execution.
    if [[ "${_TYPO_DEBUG_ACTIVE:-0}" -eq 1 ]]; then
        return
    fi
    _TYPO_DEBUG_ACTIVE=1

    if [[ "${TYPO_READY_FOR_PREEXEC:-0}" -eq 1 ]]; then
        case "${BASH_COMMAND:-}" in
            _typo_*)
                ;;
            *)
                TYPO_READY_FOR_PREEXEC=0
                _typo_preexec
                ;;
        esac
    fi

    if [[ -n "${_TYPO_PREV_DEBUG_TRAP:-}" ]]; then
        eval -- "$_TYPO_PREV_DEBUG_TRAP"
    fi

    _TYPO_DEBUG_ACTIVE=0
}

_typo_install_precmd_hook() {
    if [[ -z "${PROMPT_COMMAND:-}" ]]; then
        PROMPT_COMMAND="_typo_precmd"
    elif [[ "$PROMPT_COMMAND" != *"_typo_precmd"* ]]; then
        PROMPT_COMMAND="_typo_precmd; $PROMPT_COMMAND"
    fi
}

_typo_install_debug_hook() {
    local current_debug_trap

    current_debug_trap=$(trap -p DEBUG)
    if [[ "$current_debug_trap" == *"_typo_debug_trap"* ]]; then
        return
    fi

    _TYPO_PREV_DEBUG_TRAP=""
    if [[ "$current_debug_trap" =~ ^trap\ --\ \'(.*)\'\ DEBUG$ ]]; then
        _TYPO_PREV_DEBUG_TRAP="${BASH_REMATCH[1]}"
    fi

    trap '_typo_debug_trap' DEBUG
}

_typo_install_exit_hook() {
    local current_exit_trap

    current_exit_trap=$(trap -p EXIT)
    if [[ "$current_exit_trap" == *"_typo_bashexit"* ]]; then
        return
    fi

    if [[ "$current_exit_trap" =~ ^trap\ --\ \'(.*)\'\ EXIT$ ]]; then
        trap "_typo_bashexit; ${BASH_REMATCH[1]}" EXIT
    else
        trap '_typo_bashexit' EXIT
    fi
}

# Esc+Esc to fix command.
# Direct bind -x for \e\e is unreliable because readline treats \e as the
# meta-prefix in emacs mode, causing ambiguity with \e\e[C / \e\e[D and
# the built-in "complete" fallback. Work around this with macro indirection:
# \e\e fires a macro that types an internal sequence, which is bound to the
# real handler via bind -x. The internal sequence uses \C-x\C-_ (Ctrl+X
# Ctrl+Underscore) which is otherwise unused and unlikely to be typed.
bind -r '\e\e[C' 2>/dev/null
bind -r '\e\e[D' 2>/dev/null
bind -x '"\C-x\C-_":_typo_fix_command' 2>/dev/null
bind '"\e\e":"\C-x\C-_"' 2>/dev/null

_typo_init_stderr_cache
_typo_save_original_stderr
_typo_cleanup_stale_caches
_typo_install_precmd_hook
_typo_install_debug_hook
_typo_install_exit_hook

# Mark shell integration as loaded.
export TYPO_SHELL_INTEGRATION=1
