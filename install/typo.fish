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

    if test "$use_last_command" -eq 1
        set fixed (typo fix --exit-code "$last_exit_code" "$cmd" 2>/dev/null | string collect)
    else
        set fixed (typo fix --no-history "$cmd" 2>/dev/null | string collect)
    end

    if test -n "$fixed"; and test "$fixed" != "$cmd"
        commandline -r "$fixed"
        commandline -C (string length -- "$fixed")
        commandline -f repaint
    end
end

function _typo_preexec --on-event fish_preexec
    set -gx TYPO_LAST_COMMAND "$argv"
end

function _typo_postexec --on-event fish_postexec
    set -gx TYPO_LAST_EXIT_CODE $status
end

function _typo_fish_exit --on-event fish_exit
    set -e TYPO_LAST_COMMAND
    set -e TYPO_LAST_EXIT_CODE
end

bind escape,escape _typo_fix_command

if bind -M insert >/dev/null 2>/dev/null
    bind -M insert escape,escape _typo_fix_command
end

set -gx TYPO_ACTIVE_SHELL fish
set -gx TYPO_SHELL_INTEGRATION 1
