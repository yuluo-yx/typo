# typo - Command auto-correction for PowerShell
#
# Installation:
#   Add the following to $PROFILE.CurrentUserCurrentHost:
#     Invoke-Expression (& typo init powershell)
#
# Usage:
#   1. Press <Esc><Esc> before execution to fix the current command.
#   2. After a failed command, press <Esc><Esc> on an empty line to fix the previous command.
#
# Requirements:
#   - PowerShell 7+
#   - PSReadLine

if ($PSVersionTable.PSVersion.Major -lt 7) {
    Write-Error "typo PowerShell integration requires PowerShell 7+."
    return
}

if (-not (Get-Module -Name PSReadLine)) {
    try {
        Import-Module PSReadLine -ErrorAction Stop
    } catch {
        Write-Error "typo PowerShell integration requires PSReadLine. Install and import PSReadLine, then run 'Invoke-Expression (& typo init powershell)' again."
        return
    }
}

if (-not ("Microsoft.PowerShell.PSConsoleReadLine" -as [type])) {
    Write-Error "PSReadLine is not available in the current PowerShell session, so typo PowerShell integration cannot be installed."
    return
}

if (-not ($global:TYPO_PS_STATE -is [hashtable])) {
    $global:TYPO_PS_STATE = @{}
}

function global:__typo_NewStderrCache {
    $tmpDir = [System.IO.Path]::GetTempPath()
    $fileName = "typo-stderr-$PID-$([guid]::NewGuid().ToString('N')).log"
    $cachePath = [System.IO.Path]::Combine($tmpDir, $fileName)
    New-Item -ItemType File -Path $cachePath -Force | Out-Null
    return $cachePath
}

function global:__typo_EnsureStderrCache {
    $shellId = [string]$PID
    $cachePath = [string]$env:TYPO_STDERR_CACHE
    $cacheOwner = [string]$env:TYPO_STDERR_CACHE_OWNER

    if (-not [string]::IsNullOrWhiteSpace($cacheOwner) -and $cacheOwner -ne $shellId) {
        $env:TYPO_STDERR_CACHE = ""
        $env:TYPO_STDERR_CACHE_OWNER = ""
        $cachePath = ""
    }

    if (-not [string]::IsNullOrWhiteSpace($cachePath) -and (Test-Path -LiteralPath $cachePath)) {
        return $cachePath
    }

    $cachePath = __typo_NewStderrCache
    $env:TYPO_STDERR_CACHE = $cachePath
    $env:TYPO_STDERR_CACHE_OWNER = $shellId

    return $cachePath
}

function global:__typo_ClearStderrCache {
    $cachePath = __typo_EnsureStderrCache
    [System.IO.File]::WriteAllText($cachePath, "")
    return $cachePath
}

function global:__typo_CleanupStaleCaches {
    $tmpDir = [System.IO.Path]::GetTempPath()
    $cutoff = (Get-Date).AddDays(-1)
    $currentCache = [string]$env:TYPO_STDERR_CACHE

    Get-ChildItem -Path $tmpDir -Filter "typo-stderr-*" -File -ErrorAction SilentlyContinue | ForEach-Object {
        if ($_.FullName -eq $currentCache) {
            return
        }
        if ($_.LastWriteTime -lt $cutoff) {
            Remove-Item -LiteralPath $_.FullName -Force -ErrorAction SilentlyContinue
        }
    }
}

function global:__typo_RemoveCurrentCache {
    $cachePath = [string]$env:TYPO_STDERR_CACHE
    if (-not [string]::IsNullOrWhiteSpace($cachePath) -and
        [string]$env:TYPO_STDERR_CACHE_OWNER -eq [string]$PID -and
        (Test-Path -LiteralPath $cachePath)) {
        Remove-Item -LiteralPath $cachePath -Force -ErrorAction SilentlyContinue
    }

    $env:TYPO_STDERR_CACHE = ""
    $env:TYPO_STDERR_CACHE_OWNER = ""
}

function global:__typo_GetBufferState {
    $line = ""
    $cursor = 0
    [Microsoft.PowerShell.PSConsoleReadLine]::GetBufferState([ref]$line, [ref]$cursor)
    return @{
        Line   = [string]$line
        Cursor = [int]$cursor
    }
}

function global:__typo_SetBufferState([string]$line) {
    $buffer = __typo_GetBufferState
    [Microsoft.PowerShell.PSConsoleReadLine]::Replace(0, $buffer.Line.Length, $line)
    [Microsoft.PowerShell.PSConsoleReadLine]::SetCursorPosition($line.Length)
}

function global:__typo_ShouldWrapAcceptedLine([string]$line) {
    if ([string]::IsNullOrWhiteSpace($line)) {
        return $false
    }

    $tokens = $null
    $parseErrors = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseInput($line, [ref]$tokens, [ref]$parseErrors)
    if ($parseErrors.Count -gt 0) {
        return $false
    }

    $commandAst = $ast.Find({ param($node) $node -is [System.Management.Automation.Language.CommandAst] }, $true)
    if ($null -eq $commandAst) {
        return $false
    }

    $commandName = $commandAst.GetCommandName()
    if ([string]::IsNullOrWhiteSpace($commandName)) {
        return $false
    }

    $commandInfo = Get-Command -Name $commandName -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $commandInfo) {
        return $false
    }

    return $commandInfo.CommandType -in @("Application", "ExternalScript")
}

function global:__typo_InvokeFix([string]$command, [bool]$useLastCommand) {
    if ([string]::IsNullOrWhiteSpace($command)) {
        return ""
    }

    $fixed = ""
    $lastExitCode = 0
    if ($global:TYPO_PS_STATE.ContainsKey("LastExitCode")) {
        $lastExitCode = [int]$global:TYPO_PS_STATE.LastExitCode
    }

    if ($useLastCommand) {
        $cachePath = __typo_EnsureStderrCache
        $hasStderr = (Test-Path -LiteralPath $cachePath) -and ((Get-Item -LiteralPath $cachePath).Length -gt 0)
        if ($hasStderr) {
            $fixed = & typo fix --exit-code $lastExitCode -s $cachePath $command 2>$null
        } else {
            $fixed = & typo fix --exit-code $lastExitCode $command 2>$null
        }
    } else {
        $fixed = & typo fix --no-history $command 2>$null
    }

    return [string]$fixed
}

function global:__typo_FixCommand {
    $buffer = __typo_GetBufferState
    $command = [string]$buffer.Line
    $useLastCommand = $false

    if ([string]::IsNullOrWhiteSpace($command)) {
        $useLastCommand = $true
        $command = [string]$global:TYPO_PS_STATE.LastAcceptedLine
    }

    if ([string]::IsNullOrWhiteSpace($command)) {
        return
    }

    $fixed = __typo_InvokeFix -command $command -useLastCommand:$useLastCommand
    $fixed = $fixed.TrimEnd("`r", "`n")

    if (-not [string]::IsNullOrWhiteSpace($fixed) -and $fixed -ne $command) {
        __typo_SetBufferState -line $fixed
    }
}

function global:__typo_InvokeAcceptedLine {
    $line = [string]$global:TYPO_PS_STATE.PendingOriginalLine
    $global:TYPO_PS_STATE.PendingOriginalLine = ""

    if ([string]::IsNullOrWhiteSpace($line)) {
        return
    }

    $cachePath = __typo_ClearStderrCache
    $commandSucceeded = $true
    $commandExitCode = 0

    & { Invoke-Expression $line } 2> $cachePath
    $commandSucceeded = $?
    $commandExitCode = $LASTEXITCODE

    if (Test-Path -LiteralPath $cachePath) {
        Get-Content -LiteralPath $cachePath -ErrorAction SilentlyContinue | ForEach-Object {
            [Console]::Error.WriteLine($_)
        }
    }

    if ($null -ne $commandExitCode) {
        Set-Variable -Scope Global -Name LASTEXITCODE -Value $commandExitCode -Force
    }

    $global:TYPO_PS_STATE.LastSucceeded = $commandSucceeded
    if ($null -ne $commandExitCode -and $commandExitCode -ne 0) {
        $global:TYPO_PS_STATE.LastExitCode = [int]$commandExitCode
    } elseif ($commandSucceeded) {
        $global:TYPO_PS_STATE.LastExitCode = 0
    } else {
        $global:TYPO_PS_STATE.LastExitCode = 1
    }
    $global:TYPO_PS_STATE.SkipNextPromptCapture = $true
    $env:TYPO_LAST_EXIT_CODE = [string]$global:TYPO_PS_STATE.LastExitCode
}

function global:__typo_AcceptLine {
    $buffer = __typo_GetBufferState
    $line = [string]$buffer.Line

    if (-not [string]::IsNullOrWhiteSpace($line)) {
        $global:TYPO_PS_STATE.LastAcceptedLine = $line
    }

    if (__typo_ShouldWrapAcceptedLine -line $line) {
        [Microsoft.PowerShell.PSConsoleReadLine]::AddToHistory($line)
        $global:TYPO_PS_STATE.PendingOriginalLine = $line
        __typo_ClearStderrCache | Out-Null

        $wrapper = "__typo_InvokeAcceptedLine"
        [Microsoft.PowerShell.PSConsoleReadLine]::Replace(0, $line.Length, $wrapper)
        [Microsoft.PowerShell.PSConsoleReadLine]::SetCursorPosition($wrapper.Length)
    } else {
        $global:TYPO_PS_STATE.PendingOriginalLine = ""
    }

    [Microsoft.PowerShell.PSConsoleReadLine]::AcceptLine()
}

function global:__typo_InvokeOriginalPrompt {
    $promptScript = $global:TYPO_PS_STATE.OriginalPrompt
    if ($promptScript -is [scriptblock]) {
        return & $promptScript
    }

    return "PS $($executionContext.SessionState.Path.CurrentLocation)$('>' * ($nestedPromptLevel + 1)) "
}

if (-not $global:TYPO_PS_STATE.ContainsKey("OriginalPrompt")) {
    $global:TYPO_PS_STATE.OriginalPrompt = ${function:prompt}
}

function global:prompt {
    if (-not $global:TYPO_PS_STATE.SkipNextPromptCapture) {
        $commandSucceeded = $?
        $commandExitCode = $LASTEXITCODE

        $global:TYPO_PS_STATE.LastSucceeded = $commandSucceeded
        if ($null -ne $commandExitCode -and $commandExitCode -ne 0) {
            $global:TYPO_PS_STATE.LastExitCode = [int]$commandExitCode
        } elseif ($commandSucceeded) {
            $global:TYPO_PS_STATE.LastExitCode = 0
        } else {
            $global:TYPO_PS_STATE.LastExitCode = 1
        }
        $env:TYPO_LAST_EXIT_CODE = [string]$global:TYPO_PS_STATE.LastExitCode
    } else {
        $global:TYPO_PS_STATE.SkipNextPromptCapture = $false
    }

    return __typo_InvokeOriginalPrompt
}

$historyHandler = {
    param([string]$line)

    if ($line -eq "__typo_InvokeAcceptedLine") {
        return "SkipAdding"
    }

    $originalHandler = $global:TYPO_PS_STATE.OriginalAddToHistoryHandler
    if ($originalHandler -is [scriptblock]) {
        return & $originalHandler $line
    }

    return $true
}

if (-not $global:TYPO_PS_STATE.ContainsKey("OriginalAddToHistoryHandler")) {
    $global:TYPO_PS_STATE.OriginalAddToHistoryHandler = (Get-PSReadLineOption).AddToHistoryHandler
}

Set-PSReadLineOption -AddToHistoryHandler $historyHandler
Set-PSReadLineKeyHandler -Chord Escape,Escape -BriefDescription "typo-fix-command" -ScriptBlock {
    param($key, $arg)
    __typo_FixCommand
}
Set-PSReadLineKeyHandler -Chord Enter -BriefDescription "typo-accept-line" -ScriptBlock {
    param($key, $arg)
    __typo_AcceptLine
}

if ($global:TYPO_PS_STATE.ExitSubscriptionId) {
    Unregister-Event -SubscriptionId $global:TYPO_PS_STATE.ExitSubscriptionId -ErrorAction SilentlyContinue
}

$subscription = Register-EngineEvent -SourceIdentifier PowerShell.Exiting -Action {
    __typo_RemoveCurrentCache
}
$global:TYPO_PS_STATE.ExitSubscriptionId = $subscription.SubscriptionId

$global:TYPO_PS_STATE.LastAcceptedLine = [string]$global:TYPO_PS_STATE.LastAcceptedLine
$global:TYPO_PS_STATE.LastExitCode = 0
$global:TYPO_PS_STATE.LastSucceeded = $true
$global:TYPO_PS_STATE.SkipNextPromptCapture = $false
$global:TYPO_PS_STATE.PendingOriginalLine = ""

$env:TYPO_ACTIVE_SHELL = "powershell"
$env:TYPO_SHELL_INTEGRATION = "1"
$env:TYPO_LAST_EXIT_CODE = "0"

__typo_EnsureStderrCache | Out-Null
__typo_CleanupStaleCaches
