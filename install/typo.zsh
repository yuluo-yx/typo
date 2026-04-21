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
    local alias_context_file=""
    local -a alias_args

    # If buffer is empty, get last command from history
    if [[ -z "$cmd" ]]; then
        use_last_command=1
        cmd=$(fc -ln -1 | sed 's/^[[:space:]]*//')
    fi

    [[ -z "$cmd" ]] && return

    _typo_write_alias_context "$cmd"
    if _typo_owns_alias_context && [[ -f "$TYPO_ALIAS_CONTEXT" ]]; then
        alias_context_file="$TYPO_ALIAS_CONTEXT"
        alias_args=(--alias-context "$alias_context_file")
    fi

    if [[ "$use_last_command" -eq 1 && -f "$stderr_file" && -s "$stderr_file" ]]; then
        fixed=$(typo fix "${alias_args[@]}" --exit-code "$last_exit_code" -s "$stderr_file" "$cmd" 2>/dev/null)
    elif [[ "$use_last_command" -eq 1 ]]; then
        fixed=$(typo fix "${alias_args[@]}" --exit-code "$last_exit_code" "$cmd" 2>/dev/null)
    else
        # Preview fixes for the current buffer should not pollute history.
        # Only fixes applied after a failed command should be persisted.
        fixed=$(typo fix "${alias_args[@]}" --no-history "$cmd" 2>/dev/null)
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

_typo_owns_alias_context() {
    [[ -n "${TYPO_ALIAS_CONTEXT:-}" && "${TYPO_ALIAS_CONTEXT_OWNER:-}" == "$(_typo_current_shell_id)" ]]
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

# Initialize the alias context file for the current shell session.
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
        : >| "$alias_context" 2>/dev/null || return 1
    fi
    if [[ ! -f "$alias_context" || ! -w "$alias_context" ]]; then
        return 1
    fi

    TYPO_ALIAS_CONTEXT="$alias_context"
    TYPO_ALIAS_CONTEXT_OWNER="$shell_id"
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

    for file in "$tmp_dir"/typo-alias-*(.Nm+1); do
        [[ -O "$file" ]] || continue
        [[ "$file" == "${TYPO_ALIAS_CONTEXT:-}" ]] && continue
        rm -f -- "$file" 2>/dev/null
    done
}

_typo_simple_function_expansion() {
    emulate -L zsh
    setopt extended_glob

    local definition="$1"
    local line trimmed expansion suffix
    local count=0

    for line in "${(@f)definition}"; do
        trimmed="${line##[[:space:]]##}"
        trimmed="${trimmed%%[[:space:]]##}"
        [[ -z "$trimmed" || "$trimmed" == "{" || "$trimmed" == "}" || "$trimmed" == *"() {"* || "$trimmed" == "function "* ]] && continue
        expansion="$trimmed"
        count=$((count + 1))
    done

    [[ "$count" -eq 1 ]] || return 1
    expansion="${expansion%;}"
    suffix=' "$@"'
    [[ "$expansion" == *"$suffix" ]] && expansion="${expansion%$suffix}"
    suffix=' $@'
    [[ "$expansion" == *"$suffix" ]] && expansion="${expansion%$suffix}"
    [[ -n "$expansion" ]] || return 1
    [[ "$expansion" == *'|'* || "$expansion" == *'&'* || "$expansion" == *';'* || "$expansion" == *'<'* || "$expansion" == *'>'* || "$expansion" == *'$'* || "$expansion" == *'`'* || "$expansion" == *'('* || "$expansion" == *')'* || "$expansion" == *'{'* || "$expansion" == *'}'* || "$expansion" == *'['* || "$expansion" == *']'* ]] && return 1

    print -r -- "$expansion"
}

_typo_is_command_separator() {
    [[ "$1" == '&&' || "$1" == '||' || "$1" == '|' || "$1" == ';' || "$1" == '&' ]]
}

_typo_collect_command_words() {
    emulate -L zsh

    local raw="$1"
    local -a tokens results
    local idx=1
    local token=""
    local expect_value=0

    tokens=(${(z)raw})
    while (( idx <= $#tokens )); do
        while (( idx <= $#tokens )) && _typo_is_command_separator "${tokens[idx]}"; do
            (( idx++ ))
        done
        (( idx > $#tokens )) && break

        while (( idx <= $#tokens )); do
            token="${tokens[idx]}"
            if _typo_is_command_separator "$token"; then
                break
            fi

            case "$token" in
                builtin|nocorrect|noglob)
                    (( idx++ ))
                    continue
                    ;;
                command)
                    (( idx++ ))
                    while (( idx <= $#tokens )); do
                        token="${tokens[idx]}"
                        [[ "$token" == '--' ]] && { (( idx++ )); break; }
                        [[ "$token" == -* ]] || break
                        (( idx++ ))
                    done
                    continue
                    ;;
                env)
                    (( idx++ ))
                    expect_value=0
                    while (( idx <= $#tokens )); do
                        token="${tokens[idx]}"
                        if (( expect_value )); then
                            expect_value=0
                            (( idx++ ))
                            continue
                        fi
                        [[ "$token" == '--' ]] && { (( idx++ )); break; }
                        case "$token" in
                            *=*|--debug|-v|--ignore-environment|-i|--null|-0)
                                (( idx++ ))
                                continue
                                ;;
                            --argv0=*|--chdir=*|--default-signal=*|--ignore-signal=*|--block-signal=*|--signal=*|--unset=*|--split-string=*)
                                (( idx++ ))
                                continue
                                ;;
                            --argv0|--chdir|--default-signal|--ignore-signal|--block-signal|--signal|--unset|--split-string|-C|-S|-u)
                                expect_value=1
                                (( idx++ ))
                                continue
                                ;;
                        esac
                        break
                    done
                    continue
                    ;;
                sudo)
                    (( idx++ ))
                    expect_value=0
                    while (( idx <= $#tokens )); do
                        token="${tokens[idx]}"
                        if (( expect_value )); then
                            expect_value=0
                            (( idx++ ))
                            continue
                        fi
                        [[ "$token" == '--' ]] && { (( idx++ )); break; }
                        case "$token" in
                            --close-from=*|--group=*|--host=*|--other-user=*|--preserve-env=*|--prompt=*|--role=*|--user=*|--chdir=*)
                                (( idx++ ))
                                continue
                                ;;
                            --askpass|--background|--non-interactive|--preserve-env|--remove-timestamp|--reset-timestamp|--set-home|--shell|--stdin|--validate|--login|-A|-b|-E|-e|-H|-i|-k|-K|-l|-n|-P|-s|-v)
                                (( idx++ ))
                                continue
                                ;;
                            --close-from|--group|--host|--other-user|--preserve-env|--prompt|--role|--user|--chdir|-C|-g|-h|-p|-R|-r|-T|-u)
                                expect_value=1
                                (( idx++ ))
                                continue
                                ;;
                        esac
                        break
                    done
                    continue
                    ;;
                time)
                    (( idx++ ))
                    while (( idx <= $#tokens )) && [[ "${tokens[idx]}" == -p ]]; do
                        (( idx++ ))
                    done
                    continue
                    ;;
            esac

            results+=("$token")
            break
        done

        while (( idx <= $#tokens )) && ! _typo_is_command_separator "${tokens[idx]}"; do
            (( idx++ ))
        done
    done

    printf '%s\n' "${results[@]}"
}

_typo_append_alias_context_entry() {
    emulate -L zsh

    local name="$1"
    local expansion=""
    local definition=""
    local -a expansion_tokens

    [[ -n "$name" ]] || return
    if [[ -n "${_TYPO_ALIAS_CONTEXT_SEEN[$name]:-}" ]]; then
        return
    fi
    _TYPO_ALIAS_CONTEXT_SEEN[$name]=1

    if (( ${+aliases[$name]} )); then
        expansion="${aliases[$name]}"
        [[ -n "$expansion" ]] || return
        printf 'zsh\talias\t%s\t%s\n' "$name" "$expansion" >> "$TYPO_ALIAS_CONTEXT"
    elif (( ${+functions[$name]} )); then
        definition="$(functions "$name" 2>/dev/null)" || return
        expansion="$(_typo_simple_function_expansion "$definition")" || return
        [[ -n "$expansion" ]] || return
        printf 'zsh\tfunction\t%s\t%s\n' "$name" "$expansion" >> "$TYPO_ALIAS_CONTEXT"
    else
        return
    fi

    expansion_tokens=(${(z)expansion})
    (( $#expansion_tokens > 0 )) || return
    _typo_append_alias_context_entry "${expansion_tokens[1]}"
}

_typo_write_alias_context() {
    emulate -L zsh

    local raw="$1"
    local name=""
    local -A _TYPO_ALIAS_CONTEXT_SEEN

    _typo_init_alias_context || return
    : >| "$TYPO_ALIAS_CONTEXT" 2>/dev/null || return

    while IFS= read -r name; do
        [[ -n "$name" ]] || continue
        _typo_append_alias_context_entry "$name"
    done < <(_typo_collect_command_words "$raw")

    if [[ ! -s "$TYPO_ALIAS_CONTEXT" ]]; then
        : >| "$TYPO_ALIAS_CONTEXT" 2>/dev/null
    fi
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

    if _typo_owns_alias_context; then
        rm -f -- "$TYPO_ALIAS_CONTEXT" 2>/dev/null
    fi
    unset TYPO_ALIAS_CONTEXT
    unset TYPO_ALIAS_CONTEXT_OWNER
}

autoload -Uz add-zsh-hook
add-zsh-hook -D preexec _typo_preexec 2>/dev/null
add-zsh-hook -D precmd _typo_precmd 2>/dev/null
add-zsh-hook -D zshexit _typo_zshexit 2>/dev/null
add-zsh-hook preexec _typo_preexec
add-zsh-hook precmd _typo_precmd
add-zsh-hook zshexit _typo_zshexit

_typo_init_stderr_cache
_typo_init_alias_context
_typo_save_original_stderr
_typo_cleanup_stale_caches

# Mark shell integration as loaded
export TYPO_SHELL_INTEGRATION=1
