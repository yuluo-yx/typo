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
    $currentShellPid = [string]$PID
    $cachePath = [string]$env:TYPO_STDERR_CACHE
    $cacheOwner = [string]$env:TYPO_STDERR_CACHE_OWNER

    if (-not [string]::IsNullOrWhiteSpace($cacheOwner) -and $cacheOwner -ne $currentShellPid) {
        $env:TYPO_STDERR_CACHE = ""
        $env:TYPO_STDERR_CACHE_OWNER = ""
        $cachePath = ""
    }

    if (-not [string]::IsNullOrWhiteSpace($cachePath) -and (Test-Path -LiteralPath $cachePath)) {
        return $cachePath
    }

    $cachePath = __typo_NewStderrCache
    $env:TYPO_STDERR_CACHE = $cachePath
    $env:TYPO_STDERR_CACHE_OWNER = $currentShellPid

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

function global:__typo_NewAliasContext {
    $tmpDir = [System.IO.Path]::GetTempPath()
    $fileName = "typo-alias-$PID-$([guid]::NewGuid().ToString('N')).tsv"
    $contextPath = [System.IO.Path]::Combine($tmpDir, $fileName)
    New-Item -ItemType File -Path $contextPath -Force | Out-Null
    return $contextPath
}

function global:__typo_EnsureAliasContext {
    $currentShellPid = [string]$PID
    $contextPath = [string]$env:TYPO_ALIAS_CONTEXT
    $contextOwner = [string]$env:TYPO_ALIAS_CONTEXT_OWNER

    if (-not [string]::IsNullOrWhiteSpace($contextOwner) -and $contextOwner -ne $currentShellPid) {
        $env:TYPO_ALIAS_CONTEXT = ""
        $env:TYPO_ALIAS_CONTEXT_OWNER = ""
        $contextPath = ""
    }

    if (-not [string]::IsNullOrWhiteSpace($contextPath) -and (Test-Path -LiteralPath $contextPath)) {
        return $contextPath
    }

    $contextPath = __typo_NewAliasContext
    $env:TYPO_ALIAS_CONTEXT = $contextPath
    $env:TYPO_ALIAS_CONTEXT_OWNER = $currentShellPid

    return $contextPath
}

function global:__typo_RemoveAliasContext {
    $contextPath = [string]$env:TYPO_ALIAS_CONTEXT
    if (-not [string]::IsNullOrWhiteSpace($contextPath) -and
        [string]$env:TYPO_ALIAS_CONTEXT_OWNER -eq [string]$PID -and
        (Test-Path -LiteralPath $contextPath)) {
        Remove-Item -LiteralPath $contextPath -Force -ErrorAction SilentlyContinue
    }

    $env:TYPO_ALIAS_CONTEXT = ""
    $env:TYPO_ALIAS_CONTEXT_OWNER = ""
}

function global:__typo_CleanupStaleAliasContexts {
    $tmpDir = [System.IO.Path]::GetTempPath()
    $cutoff = (Get-Date).AddDays(-1)
    $currentContext = [string]$env:TYPO_ALIAS_CONTEXT

    Get-ChildItem -Path $tmpDir -Filter "typo-alias-*" -File -ErrorAction SilentlyContinue | ForEach-Object {
        if ($_.FullName -eq $currentContext) {
            return
        }
        if ($_.LastWriteTime -lt $cutoff) {
            Remove-Item -LiteralPath $_.FullName -Force -ErrorAction SilentlyContinue
        }
    }
}

function global:__typo_IsSafeAliasToken([string]$value) {
    return -not [string]::IsNullOrWhiteSpace($value) -and $value -notmatch "[\t\r\n\0]"
}

function global:__typo_GetSimpleFunctionExpansion([string]$definition) {
    if ([string]::IsNullOrWhiteSpace($definition)) {
        return ""
    }

    $line = [string]$definition.Trim()
    $line = $line -replace '\s+@args$', ''
    $line = $line -replace '\s+\$args$', ''
    if ($line.Contains("`n") -or $line.Contains(";") -or $line -match '[\|&<>`$(){}\[\]]') {
        return ""
    }

    return $line.Trim()
}

function global:__typo_WriteAliasContext {
    $contextPath = __typo_EnsureAliasContext
    if ([string]::IsNullOrWhiteSpace($contextPath)) {
        return ""
    }

    [System.IO.File]::WriteAllText($contextPath, "")
    $lines = [System.Collections.Generic.List[string]]::new()

    Get-Alias -ErrorAction SilentlyContinue | ForEach-Object {
        $name = [string]$_.Name
        $expansion = [string]$_.Definition
        if ((__typo_IsSafeAliasToken $name) -and (__typo_IsSafeAliasToken $expansion)) {
            $lines.Add("powershell`talias`t$name`t$expansion")
        }
    }

    Get-Command -CommandType Function -ErrorAction SilentlyContinue | ForEach-Object {
        $name = [string]$_.Name
        if ($name.StartsWith("__typo_")) {
            return
        }
        $expansion = __typo_GetSimpleFunctionExpansion ([string]$_.Definition)
        if ((__typo_IsSafeAliasToken $name) -and (__typo_IsSafeAliasToken $expansion)) {
            $lines.Add("powershell`tfunction`t$name`t$expansion")
        }
    }

    if ($lines.Count -gt 0) {
        [System.IO.File]::WriteAllLines($contextPath, $lines)
    }

    return $contextPath
}

function global:__typo_GetBufferState {
    $line = ""
    $cursor = 0
    try {
        [Microsoft.PowerShell.PSConsoleReadLine]::GetBufferState([ref]$line, [ref]$cursor)
    } catch {
        $line = ""
        $cursor = 0
    }
    if ($null -eq $line) { $line = "" }
    return @{
        Line   = [string]$line
        Cursor = [int]$cursor
    }
}

function global:__typo_SetBufferState([string]$line) {
    $buffer = __typo_GetBufferState
    if ($null -eq $buffer -or $null -eq $buffer.Line) { return }
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

    # Check if it's a PowerShell built-in (cmdlet, function, alias to cmdlet)
    # If so, don't wrap - let PowerShell handle it natively
    $commandInfo = Get-Command -Name $commandName -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $commandInfo) {
        # Command not found - likely a typo, wrap it to capture the error
        return $true
    }

    # Only skip wrapping for PowerShell-native command types
    return $commandInfo.CommandType -in @("Application", "ExternalScript")
}

function global:__typo_InvokeFix([string]$command, [bool]$useLastCommand) {
    if ([string]::IsNullOrWhiteSpace($command)) {
        return ""
    }

    $fixed = $null
    $lastExitCode = 0
    if ($global:TYPO_PS_STATE.ContainsKey("LastExitCode")) {
        $lastExitCode = [int]$global:TYPO_PS_STATE.LastExitCode
    }

    $aliasArgs = @()
    $aliasContextPath = __typo_WriteAliasContext
    if (-not [string]::IsNullOrWhiteSpace($aliasContextPath) -and (Test-Path -LiteralPath $aliasContextPath)) {
        $aliasArgs = @("--alias-context", $aliasContextPath)
    }

    if ($useLastCommand) {
        $cachePath = __typo_EnsureStderrCache
        $hasStderr = (Test-Path -LiteralPath $cachePath) -and ((Get-Item -LiteralPath $cachePath).Length -gt 0)
        if ($hasStderr) {
            $fixed = & typo fix @aliasArgs --exit-code $lastExitCode -s $cachePath $command 2>$null
        } else {
            $fixed = & typo fix @aliasArgs --exit-code $lastExitCode $command 2>$null
        }
     } else {
        $fixed = & typo fix @aliasArgs --no-history $command 2>$null
    }

    if ($null -eq $fixed) {
        return ""
    }

    # If typo returns multiple lines, join.
    if ($fixed -is [array]) {
        $fixed = $fixed -join "`n"
    }

    return [string]$fixed
}


function global:__typo_FixCommand {
    $buffer = __typo_GetBufferState

    # add null check
    if ($null -eq $buffer) { return }
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

    if ($null -eq $fixed) {
        return
    }

    $fixed = [string]$fixed
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

    # Use a child PowerShell to fully isolate error display
    $psCommand = [powershell]::Create([System.Management.Automation.RunspaceMode]::CurrentRunspace)
    try {
        $null = $psCommand.AddScript($line)
        $output = $psCommand.Invoke()

        # Pass stdout through
        if ($null -ne $output) {
            foreach ($item in $output) {
                $item
            }
        }

        $stderrLines = [System.Collections.Generic.List[string]]::new()

        # Collect errors without displaying them
        if ($psCommand.HadErrors -and $psCommand.Streams.Error.Count -gt 0) {
            $commandSucceeded = $false
            foreach ($err in $psCommand.Streams.Error) {
                $stderrLines.Add($err.ToString())
            }
        }

        $commandExitCode = $LASTEXITCODE

        if ($stderrLines.Count -gt 0) {
            [System.IO.File]::WriteAllLines($cachePath, $stderrLines)
            foreach ($errLine in $stderrLines) {
                [Console]::Error.WriteLine($errLine)
            }
            if ($null -eq $commandExitCode -or $commandExitCode -eq 0) {
                $commandExitCode = 1
            }
        }

        if (-not $commandSucceeded -and ($null -eq $commandExitCode -or $commandExitCode -eq 0)) {
            $commandExitCode = 1
        }
    } catch {
        $commandSucceeded = $false
        $commandExitCode = if ($null -ne $LASTEXITCODE -and $LASTEXITCODE -ne 0) { $LASTEXITCODE } else { 1 }
        $errMsg = $_.Exception.Message
        if ([string]::IsNullOrWhiteSpace($errMsg)) {
            $errMsg = $_.ToString()
        }
        [Console]::Error.WriteLine($errMsg)
        [System.IO.File]::WriteAllText($cachePath, $errMsg)
    } finally {
        if ($null -ne $psCommand) {
            $psCommand.Dispose()
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
    
    # add null check.
    if ($null -eq $buffer) {
        [Microsoft.PowerShell.PSConsoleReadLine]::AcceptLine()
        return
    }
    $line = [string]$buffer.Line

    if (-not [string]::IsNullOrWhiteSpace($line)) {
        $global:TYPO_PS_STATE.LastAcceptedLine = $line
    }

    __typo_ClearStderrCache | Out-Null
    $global:TYPO_PS_STATE.PendingOriginalLine = ""
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

        if (-not $commandSucceeded) {
            $cachePath = __typo_EnsureStderrCache
            if ($null -ne $global:error -and $global:error.Count -gt 0) {
                [System.IO.File]::WriteAllText($cachePath, $global:error[0].ToString())
            }
        }

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

# two esc.
Set-PSReadLineKeyHandler -Chord "Escape,Escape" -BriefDescription "typo-fix-command" -ScriptBlock {
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
    __typo_RemoveAliasContext
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
__typo_EnsureAliasContext | Out-Null
__typo_CleanupStaleCaches
__typo_CleanupStaleAliasContexts
