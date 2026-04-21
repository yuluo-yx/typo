# typo - Command auto-correction for fish
#
# Installation:
#   Add to ~/.config/fish/config.fish:
#     source /path/to/typo/install/typo.fish
#
#   Or use:
#     typo init fish | source
#
# Usage:
#   1. Type a wrong command, press <Esc><Esc> to fix before executing
#   2. With an empty prompt, press <Esc><Esc> to fix the last command

function _typo_fix_command
    set -l cmd (commandline -b)
    set -l fixed ""
    set -l last_exit_code "$TYPO_LAST_EXIT_CODE"
    set -l use_last_command 0
    set -l alias_args

    if test -z "$last_exit_code"
        set last_exit_code 0
    end

    if test -z "$cmd"
        set use_last_command 1
        set cmd (history search --max 1 | string trim --left | string collect)
    end

    if test -z "$cmd"
        return
    end

    _typo_write_alias_context
    if test -n "$TYPO_ALIAS_CONTEXT"; and test -f "$TYPO_ALIAS_CONTEXT"
        set alias_args --alias-context "$TYPO_ALIAS_CONTEXT"
    end

    if test "$use_last_command" -eq 1
        set fixed (typo fix $alias_args --exit-code "$last_exit_code" "$cmd" 2>/dev/null | string collect)
    else
        set fixed (typo fix $alias_args --no-history "$cmd" 2>/dev/null | string collect)
    end

    if test -n "$fixed"; and test "$fixed" != "$cmd"
        commandline -r "$fixed"
        commandline -C (string length -- "$fixed")
        commandline -f repaint
    end
end

function _typo_current_shell_id
    echo "$fish_pid"
end

function _typo_owns_alias_context
    test -n "$TYPO_ALIAS_CONTEXT"; and test "$TYPO_ALIAS_CONTEXT_OWNER" = (_typo_current_shell_id)
end

function _typo_init_alias_context
    set -l tmp_dir /tmp
    if test -n "$TMPDIR"
        set tmp_dir "$TMPDIR"
    end

    set -l shell_id (_typo_current_shell_id)
    if test -n "$TYPO_ALIAS_CONTEXT_OWNER"; and test "$TYPO_ALIAS_CONTEXT_OWNER" != "$shell_id"
        set -e TYPO_ALIAS_CONTEXT
        set -e TYPO_ALIAS_CONTEXT_OWNER
    end

    if _typo_owns_alias_context; and test -f "$TYPO_ALIAS_CONTEXT"; and test -w "$TYPO_ALIAS_CONTEXT"
        return 0
    end

    set -e TYPO_ALIAS_CONTEXT
    set -e TYPO_ALIAS_CONTEXT_OWNER

    set -l alias_context ""
    if command -q mktemp
        set alias_context (mktemp "$tmp_dir/typo-alias-XXXXXX" 2>/dev/null)
    end
    if test -z "$alias_context"
        set alias_context "$tmp_dir/typo-alias-$fish_pid"
        printf "" > "$alias_context" 2>/dev/null; or return 1
    end
    if not test -f "$alias_context"; or not test -w "$alias_context"
        return 1
    end

    set -gx TYPO_ALIAS_CONTEXT "$alias_context"
    set -gx TYPO_ALIAS_CONTEXT_OWNER "$shell_id"
end

function _typo_fish_simple_function_expansion
    set -l definition "$argv[1]"
    set -l commands

    for line in (string split \n -- "$definition")
        set -l trimmed (string trim -- "$line")
        if test -z "$trimmed"; or test "$trimmed" = end
            continue
        end
        if string match -q '#*' -- "$trimmed"
            continue
        end
        if string match -q 'function *' -- "$trimmed"
            continue
        end
        set commands $commands "$trimmed"
    end

    if test (count $commands) -ne 1
        return 1
    end

    set -l expansion "$commands[1]"
    set expansion (string replace -r ';\s*$' '' -- "$expansion")
    set expansion (string replace -r '\s+"\$argv"$' '' -- "$expansion")
    set expansion (string replace -r '\s+\$argv$' '' -- "$expansion")
    if test -z "$expansion"
        return 1
    end
    for marker in '|' '&' ';' '<' '>' '$' '`' '(' ')' '{' '}' '[' ']'
        if string match -q "*$marker*" -- "$expansion"
            return 1
        end
    end

    echo "$expansion"
end

function _typo_write_alias_context
    _typo_init_alias_context; or return
    printf "" > "$TYPO_ALIAS_CONTEXT" 2>/dev/null; or return

    if type -q abbr
        for line in (abbr --show 2>/dev/null)
            set -l fields (string split ' ' -- "$line")
            set -l marker_index (contains -i -- -- $fields)
            if test -z "$marker_index"
                continue
            end
            set -l name_index (math $marker_index + 1)
            set -l expansion_index (math $marker_index + 2)
            if test (count $fields) -lt $expansion_index
                continue
            end
            set -l name "$fields[$name_index]"
            set -l expansion (string join ' ' $fields[$expansion_index..-1])
            if test -n "$name"; and test -n "$expansion"
                printf 'fish\tabbr\t%s\t%s\n' "$name" "$expansion" >> "$TYPO_ALIAS_CONTEXT"
            end
        end
    end

    for name in (functions -n)
        if string match -q '_typo_*' -- "$name"
            continue
        end
        set -l definition (functions "$name" 2>/dev/null | string collect)
        set -l expansion (_typo_fish_simple_function_expansion "$definition")
        if test -n "$expansion"
            printf 'fish\tfunction\t%s\t%s\n' "$name" "$expansion" >> "$TYPO_ALIAS_CONTEXT"
        end
    end

    for line in (env 2>/dev/null)
        set -l fields (string split -m 1 '=' -- "$line")
        set -l name "$fields[1]"
        if string match -rq '^[A-Za-z_][A-Za-z0-9_]*$' -- "$name"
            printf 'fish\tenv\t%s\t%s\n' "$name" "$name" >> "$TYPO_ALIAS_CONTEXT"
        end
    end
end

function _typo_preexec --on-event fish_preexec
    set -gx TYPO_LAST_COMMAND "$argv"
end

function _typo_postexec --on-event fish_postexec
    set -gx TYPO_LAST_EXIT_CODE $status
end

function _typo_fish_exit --on-event fish_exit
    if _typo_owns_alias_context
        rm -f -- "$TYPO_ALIAS_CONTEXT" 2>/dev/null
    end
    set -e TYPO_LAST_COMMAND
    set -e TYPO_LAST_EXIT_CODE
    set -e TYPO_ALIAS_CONTEXT
    set -e TYPO_ALIAS_CONTEXT_OWNER
end

bind escape,escape _typo_fix_command

if bind -M insert >/dev/null 2>/dev/null
    bind -M insert escape,escape _typo_fix_command
end

set -gx TYPO_ACTIVE_SHELL fish
set -gx TYPO_SHELL_INTEGRATION 1

_typo_init_alias_context
