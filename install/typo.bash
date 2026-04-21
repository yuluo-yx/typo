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
    local alias_context_file=""
    local alias_args=()

    # If buffer is empty, get last command from history.
    if [[ -z "$cmd" ]]; then
        use_last_command=1
        cmd=$(fc -ln -1 | sed 's/^[[:space:]]*//')
    fi

    [[ -z "$cmd" ]] && return

    _typo_write_alias_context
    if _typo_owns_alias_context && [[ -f "$TYPO_ALIAS_CONTEXT" ]]; then
        alias_context_file="$TYPO_ALIAS_CONTEXT"
        alias_args=(--alias-context "$alias_context_file")
    fi

    if [[ "$use_last_command" -eq 1 && -f "$stderr_file" && -s "$stderr_file" ]]; then
        fixed=$(typo fix "${alias_args[@]}" --exit-code "$last_exit_code" -s "$stderr_file" "$cmd" 2>/dev/null)
    elif [[ "$use_last_command" -eq 1 ]]; then
        fixed=$(typo fix "${alias_args[@]}" --exit-code "$last_exit_code" "$cmd" 2>/dev/null)
    else
        # Preview fixes for the current readline buffer should not be persisted.
        fixed=$(typo fix "${alias_args[@]}" --no-history "$cmd" 2>/dev/null)
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

_typo_owns_alias_context() {
    [[ -n "${TYPO_ALIAS_CONTEXT:-}" && "${TYPO_ALIAS_CONTEXT_OWNER:-}" == "$(_typo_current_shell_id)" ]]
}

_typo_owns_original_stderr_fd() {
    [[ -n "${TYPO_ORIG_STDERR_FD:-}" && "${TYPO_ORIG_STDERR_FD_OWNER:-}" == "$(_typo_current_shell_id)" ]]
}

# Initialize stderr cache file and FIFO for this shell session.
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
        # Cache already exists, but we still need to ensure FIFO exists
        if [[ -z "${_TYPO_STDERR_FIFO:-}" ]]; then
            _TYPO_STDERR_FIFO="$tmp_dir/typo-fifo-$$.fifo"
            rm -f "$_TYPO_STDERR_FIFO"
            if ! mkfifo "$_TYPO_STDERR_FIFO"; then
                echo "typo debug: mkfifo $_TYPO_STDERR_FIFO failed (cache exists)" >&2
                unset _TYPO_STDERR_FIFO
            fi
        fi
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

    # Create a named pipe (FIFO) for reliable stderr capture in bash 4.x
    # This avoids the process substitution race condition issue
    if [[ -z "${_TYPO_STDERR_FIFO:-}" ]]; then
        _TYPO_STDERR_FIFO="$tmp_dir/typo-fifo-$$.fifo"
        # Remove any existing file with the same name (could be leftover or regular file)
        rm -f "$_TYPO_STDERR_FIFO"
        if ! mkfifo "$_TYPO_STDERR_FIFO"; then
            echo "typo debug: mkfifo $_TYPO_STDERR_FIFO failed" >&2
            unset _TYPO_STDERR_FIFO
        fi
    fi
}

# Initialize alias context file for this shell session.
_typo_init_alias_context() {
    local tmp_dir="${TMPDIR:-/tmp}"
    local shell_id
    local alias_context=""

    shell_id="$(_typo_current_shell_id)"

    if [[ -n "${TYPO_ALIAS_CONTEXT:-}" && "${TYPO_ALIAS_CONTEXT_OWNER:-}" != "$shell_id" ]]; then
        unset TYPO_ALIAS_CONTEXT
        unset TYPO_ALIAS_CONTEXT_OWNER
    fi

    if _typo_owns_alias_context && [[ -f "$TYPO_ALIAS_CONTEXT" && -w "$TYPO_ALIAS_CONTEXT" ]]; then
        return 0
    fi

    unset TYPO_ALIAS_CONTEXT
    unset TYPO_ALIAS_CONTEXT_OWNER

    if command -v mktemp >/dev/null 2>&1; then
        alias_context=$(mktemp "$tmp_dir/typo-alias-XXXXXX" 2>/dev/null)
    fi
    if [[ -z "$alias_context" ]]; then
        alias_context="$tmp_dir/typo-alias-$$"
        : > "$alias_context" 2>/dev/null || return 1
    fi
    if [[ ! -f "$alias_context" || ! -w "$alias_context" ]]; then
        return 1
    fi

    TYPO_ALIAS_CONTEXT="$alias_context"
    TYPO_ALIAS_CONTEXT_OWNER="$shell_id"
}

# Remove stale caches older than one day.
_typo_cleanup_stale_caches() {
    local tmp_dir="${TMPDIR:-/tmp}"
    local file

    while IFS= read -r -d '' file; do
        [[ "$file" == "${TYPO_STDERR_CACHE:-}" ]] && continue
        rm -f -- "$file" 2>/dev/null
    done < <(find "$tmp_dir" -maxdepth 1 -type f -name 'typo-stderr-*' -mtime +1 -print0 2>/dev/null)

    while IFS= read -r -d '' file; do
        [[ "$file" == "${TYPO_ALIAS_CONTEXT:-}" ]] && continue
        rm -f -- "$file" 2>/dev/null
    done < <(find "$tmp_dir" -maxdepth 1 -type f -name 'typo-alias-*' -mtime +1 -print0 2>/dev/null)
}

_typo_simple_function_expansion() {
    local definition="$1"
    local body line expansion suffix
    local count=0

    body=$(printf '%s\n' "$definition" | sed '1d;$d' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | sed '/^$/d')
    while IFS= read -r line; do
        [[ -z "$line" || "$line" == "{" || "$line" == "}" ]] && continue
        expansion="$line"
        count=$((count + 1))
    done <<< "$body"

    [[ "$count" -eq 1 ]] || return 1
    expansion="${expansion%;}"
    suffix=' "$@"'
    if [[ "$expansion" == *"$suffix" ]]; then
        expansion="${expansion:0:${#expansion}-${#suffix}}"
    fi
    suffix=' $@'
    if [[ "$expansion" == *"$suffix" ]]; then
        expansion="${expansion:0:${#expansion}-${#suffix}}"
    fi
    [[ -n "$expansion" ]] || return 1
    case "$expansion" in
        *'|'*|*'&'*|*';'*|*'<'*|*'>'*|*'$'*|*'`'*|*'('*|*')'*|*'{'*|*'}'*|*'['*|*']'*)
            return 1
            ;;
    esac

    printf '%s\n' "$expansion"
}

_typo_write_alias_context() {
    _typo_init_alias_context || return
    : > "$TYPO_ALIAS_CONTEXT" 2>/dev/null || return

    local name expansion definition alias_line
    if declare -p BASH_ALIASES >/dev/null 2>&1; then
        for name in "${!BASH_ALIASES[@]}"; do
            expansion="${BASH_ALIASES[$name]}"
            [[ -n "$name" && -n "$expansion" ]] || continue
            printf 'bash\talias\t%s\t%s\n' "$name" "$expansion" >> "$TYPO_ALIAS_CONTEXT"
        done
    else
        while IFS= read -r alias_line; do
            name="${alias_line#alias }"
            name="${name%%=*}"
            expansion="${alias_line#*=}"
            if [[ "$expansion" == \'*\' ]]; then
                expansion="${expansion:1:${#expansion}-2}"
            fi
            [[ -n "$name" && -n "$expansion" ]] || continue
            printf 'bash\talias\t%s\t%s\n' "$name" "$expansion" >> "$TYPO_ALIAS_CONTEXT"
        done < <(alias -p 2>/dev/null)
    fi

    while read -r _ _ name; do
        [[ -n "$name" && "$name" != _typo_* ]] || continue
        definition="$(declare -f "$name" 2>/dev/null)" || continue
        expansion="$(_typo_simple_function_expansion "$definition")" || continue
        [[ -n "$expansion" ]] || continue
        printf 'bash\tfunction\t%s\t%s\n' "$name" "$expansion" >> "$TYPO_ALIAS_CONTEXT"
    done < <(declare -F)
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
    # Use named pipe (FIFO) for reliable stderr capture in bash 4.x.
    # Process substitution (>(...)) in bash 4.x has a known issue where the shell
    # waits for the tee process to complete before showing the next prompt.
    #
    # Our approach: use a named pipe with carefully ordered operations:
    # 1. Start tee reading from FIFO (it will block until FIFO is opened for write)
    # 2. Redirect stderr to FIFO (this unblocks tee)
    # 3. In precmd, close stderr->FIFO (tee gets EOF) and wait for tee to finish
    if [[ -n "${_TYPO_STDERR_FIFO:-}" && -p "${_TYPO_STDERR_FIFO:-}" ]]; then
        # Start tee in background, reading from FIFO
        tee "$TYPO_STDERR_CACHE" >&3 < "$_TYPO_STDERR_FIFO" &
        _TYPO_TEE_PID=$!
               # Now redirect stderr to the FIFO (this unblocks tee)
        exec 2> "$_TYPO_STDERR_FIFO"
    else
        # Fallback: direct redirect to cache file (no real-time stderr display)
        exec 2> "$TYPO_STDERR_CACHE"
    fi
}

# Called from PROMPT_COMMAND; capture previous command status and restore stderr.
_typo_precmd() {
    local status=$?
    TYPO_LAST_EXIT_CODE=$status

    # First restore stderr to original (closes write end of FIFO)
    # This sends EOF to tee, allowing it to terminate
    _typo_restore_stderr

    # Wait for tee process to finish writing all stderr output
    if [[ -n "${_TYPO_TEE_PID:-}" ]]; then
        wait "$_TYPO_TEE_PID" 2>/dev/null
        unset _TYPO_TEE_PID
    fi

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

    if _typo_owns_alias_context; then
        rm -f -- "$TYPO_ALIAS_CONTEXT" 2>/dev/null
    fi
    unset TYPO_ALIAS_CONTEXT
    unset TYPO_ALIAS_CONTEXT_OWNER

    # Clean up the named pipe
    if [[ -n "${_TYPO_STDERR_FIFO:-}" && -p "${_TYPO_STDERR_FIFO:-}" ]]; then
        rm -f -- "$_TYPO_STDERR_FIFO" 2>/dev/null
    fi
    unset _TYPO_STDERR_FIFO
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

_typo_install_fix_binding() {
    local bash_major="${BASH_VERSINFO[0]:-0}"

    bind -r '\e\e' 2>/dev/null
    bind -r '\e\e[C' 2>/dev/null
    bind -r '\e\e[D' 2>/dev/null
    bind -r '\C-x\C-_' 2>/dev/null

    if (( bash_major >= 5 )); then
        # Bash 5.x handles a direct Esc+Esc binding more reliably and avoids
        # the macro indirection path leaking `_typo_fix_command` into execution.
        bind -x '"\e\e":"_typo_fix_command"' 2>/dev/null
    else
        # Bash 4.x is less reliable with a direct \e\e binding because readline
        # treats \e as the meta prefix. Keep the macro indirection there and
        # shorten the timeout so the second Esc is captured promptly.
        bind 'set keyseq-timeout 50' 2>/dev/null
        bind -x '"\C-x\C-_":"_typo_fix_command"' 2>/dev/null
        bind '"\e\e":"\C-x\C-_"' 2>/dev/null
    fi
}

_typo_init_stderr_cache
_typo_init_alias_context
_typo_save_original_stderr
_typo_cleanup_stale_caches
_typo_install_precmd_hook
_typo_install_debug_hook
_typo_install_exit_hook
_typo_install_fix_binding

# Mark shell integration as loaded.
export TYPO_SHELL_INTEGRATION=1
