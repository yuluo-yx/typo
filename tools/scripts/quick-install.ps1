& {
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$script:Repo = 'yuluo-yx/typo'
$script:BinaryName = 'typo'

function Write-Usage {
    @'
Install typo on Windows with PowerShell 7+.

Usage:
  ./quick-install.ps1
  ./quick-install.ps1 -Version 0.2.0
  ./quick-install.ps1 -InstallDir C:\Users\<you>\AppData\Local\Programs\typo\bin

Environment:
  TYPO_INSTALL_VERSION           Override the Release version selector (`latest` or semver)
  TYPO_INSTALL_DIR               Override the install directory
  TYPO_INSTALL_GITHUB_API        Override the GitHub API base for tests
  TYPO_INSTALL_RELEASE_BASE_URL  Override the Release download base URL for tests
  TYPO_INSTALL_SKIP_PATH_UPDATE  Skip writing the user PATH when set to `1`
'@ | Write-Host
}

function Parse-Arguments {
    param(
        [string[]]$Arguments
    )

    $options = [ordered]@{
        Version    = if ([string]::IsNullOrWhiteSpace($env:TYPO_INSTALL_VERSION)) { 'latest' } else { $env:TYPO_INSTALL_VERSION }
        InstallDir = if ([string]::IsNullOrWhiteSpace($env:TYPO_INSTALL_DIR)) { '' } else { $env:TYPO_INSTALL_DIR }
        Help       = $false
    }

    $argsList = @($Arguments)
    for ($i = 0; $i -lt $argsList.Count; $i++) {
        $current = $argsList[$i]
        switch ($current) {
            '-Version' {
                if ($i + 1 -ge $argsList.Count) {
                    throw 'Option -Version requires a value.'
                }
                $i++
                $options.Version = $argsList[$i]
            }
            '-InstallDir' {
                if ($i + 1 -ge $argsList.Count) {
                    throw 'Option -InstallDir requires a value.'
                }
                $i++
                $options.InstallDir = $argsList[$i]
            }
            '-h' { $options.Help = $true }
            '--help' { $options.Help = $true }
            default {
                throw "Unknown argument: $current"
            }
        }
    }

    return [pscustomobject]$options
}

function Test-IsWindows {
    return [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform(
        [System.Runtime.InteropServices.OSPlatform]::Windows
    )
}

function Get-DefaultInstallDir {
    if (-not [string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
        return (Join-Path $env:LOCALAPPDATA 'Programs\typo\bin')
    }

    $localAppData = [Environment]::GetFolderPath([System.Environment+SpecialFolder]::LocalApplicationData)
    if (-not [string]::IsNullOrWhiteSpace($localAppData)) {
        return (Join-Path $localAppData 'Programs\typo\bin')
    }

    return (Join-Path (Join-Path $HOME 'AppData\Local') 'Programs\typo\bin')
}

function Normalize-VersionSelector {
    param(
        [string]$Version
    )

    if ([string]::IsNullOrWhiteSpace($Version) -or $Version -eq 'latest') {
        return 'latest'
    }

    if ($Version.StartsWith('v')) {
        return $Version
    }

    return "v$Version"
}

function Join-Url {
    param(
        [string[]]$Parts
    )

    $clean = foreach ($part in $Parts) {
        if (-not [string]::IsNullOrWhiteSpace($part)) {
            $part.TrimEnd('/')
        }
    }

    return ($clean -join '/')
}

function Get-ReleaseApiBase {
    if (-not [string]::IsNullOrWhiteSpace($env:TYPO_INSTALL_GITHUB_API)) {
        return $env:TYPO_INSTALL_GITHUB_API.TrimEnd('/')
    }

    return "https://api.github.com/repos/$script:Repo"
}

function Get-ReleaseDownloadBase {
    if (-not [string]::IsNullOrWhiteSpace($env:TYPO_INSTALL_RELEASE_BASE_URL)) {
        return $env:TYPO_INSTALL_RELEASE_BASE_URL.TrimEnd('/')
    }

    return "https://github.com/$script:Repo/releases/download"
}

function Resolve-ReleaseTag {
    param(
        [string]$VersionSelector
    )

    $normalized = Normalize-VersionSelector -Version $VersionSelector
    if ($normalized -ne 'latest') {
        return $normalized
    }

    $apiUrl = (Join-Url @((Get-ReleaseApiBase), 'releases?per_page=1'))
    $response = Invoke-RestMethod -Headers @{ Accept = 'application/vnd.github+json' } -Uri $apiUrl
    $release = @($response) | Select-Object -First 1
    if ($null -eq $release -or [string]::IsNullOrWhiteSpace($release.tag_name)) {
        throw "Unable to resolve the latest release tag from $apiUrl"
    }

    return [string]$release.tag_name
}

function Get-AssetName {
    $architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture

    switch ($architecture) {
        'X64' { return 'typo-windows-amd64.exe' }
        'Arm64' { return 'typo-windows-arm64.exe' }
        default { throw "Unsupported Windows architecture: $architecture" }
    }
}

function Download-File {
    param(
        [string]$Uri,
        [string]$OutFile
    )

    Invoke-WebRequest -Uri $Uri -OutFile $OutFile | Out-Null
}

function Get-ExpectedChecksum {
    param(
        [string]$ChecksumsFile,
        [string]$AssetName
    )

    $pattern = '^(?<hash>[A-Fa-f0-9]{64})\s+\*?(?<name>.+)$'
    foreach ($line in Get-Content -Path $ChecksumsFile) {
        if ($line -notmatch $pattern) {
            continue
        }

        if ($Matches.name.Trim() -eq $AssetName) {
            return $Matches.hash.ToLowerInvariant()
        }
    }

    throw "Unable to find checksum entry for $AssetName"
}

function Normalize-PathEntry {
    param(
        [string]$PathValue
    )

    if ([string]::IsNullOrWhiteSpace($PathValue)) {
        return ''
    }

    try {
        return ([System.IO.Path]::GetFullPath($PathValue)).TrimEnd('\').ToLowerInvariant()
    } catch {
        return $PathValue.Trim().TrimEnd('\').ToLowerInvariant()
    }
}

function Test-PathContainsDirectory {
    param(
        [string]$PathValue,
        [string]$Directory
    )

    $target = Normalize-PathEntry -PathValue $Directory
    if ([string]::IsNullOrWhiteSpace($target)) {
        return $false
    }

    foreach ($entry in ($PathValue -split ';')) {
        if ((Normalize-PathEntry -PathValue $entry) -eq $target) {
            return $true
        }
    }

    return $false
}

function Add-DirectoryToPath {
    param(
        [string]$PathValue,
        [string]$Directory
    )

    if ([string]::IsNullOrWhiteSpace($PathValue)) {
        return $Directory
    }

    return "$PathValue;$Directory"
}

function Ensure-InstallDirInPath {
    param(
        [string]$InstallDir
    )

    $processUpdated = $false
    if (-not (Test-PathContainsDirectory -PathValue $env:PATH -Directory $InstallDir)) {
        $env:PATH = Add-DirectoryToPath -PathValue $env:PATH -Directory $InstallDir
        $processUpdated = $true
    }

    $skipUserPathUpdate = ($env:TYPO_INSTALL_SKIP_PATH_UPDATE -eq '1')
    if ($skipUserPathUpdate) {
        return [pscustomobject]@{
            ProcessUpdated = $processUpdated
            UserUpdated    = $false
            Skipped        = $true
        }
    }

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $userUpdated = $false
    if (-not (Test-PathContainsDirectory -PathValue $userPath -Directory $InstallDir)) {
        $newUserPath = Add-DirectoryToPath -PathValue $userPath -Directory $InstallDir
        [Environment]::SetEnvironmentVariable('Path', $newUserPath, 'User')
        $userUpdated = $true
    }

    return [pscustomobject]@{
        ProcessUpdated = $processUpdated
        UserUpdated    = $userUpdated
        Skipped        = $false
    }
}

function Write-NextSteps {
    param(
        [string]$InstallDir,
        [string]$InstalledBinary,
        [psobject]$PathUpdate
    )

    Write-Host "Installed typo to $InstalledBinary"

    if ($PathUpdate.Skipped) {
        Write-Host "Skipped updating the user PATH because TYPO_INSTALL_SKIP_PATH_UPDATE=1."
    } elseif ($PathUpdate.UserUpdated) {
        Write-Host "Added $InstallDir to your user PATH."
    } else {
        Write-Host "$InstallDir is already present in your user PATH."
    }

    if ($PathUpdate.ProcessUpdated) {
        Write-Host 'Refreshed PATH for the current PowerShell session.'
    }

    Write-Host ''
    Write-Host 'Next steps:'
    Write-Host '  Invoke-Expression (& typo init powershell | Out-String)'
    Write-Host '  typo doctor'
    Write-Host ''
    Write-Host 'To enable typo on every PowerShell startup, add the following to $PROFILE.CurrentUserCurrentHost:'
    Write-Host "  if (!(Test-Path -Path `\$PROFILE.CurrentUserCurrentHost)) { New-Item -ItemType File -Path `\$PROFILE.CurrentUserCurrentHost -Force | Out-Null }"
    Write-Host "  Add-Content -Path `\$PROFILE.CurrentUserCurrentHost -Value 'Invoke-Expression (& typo init powershell | Out-String)'"
    Write-Host ''
    Write-Host 'If you open a new PowerShell window later, typo will already be on PATH.'
}

function Invoke-TypoQuickInstall {
    param(
        [string[]]$Arguments
    )

    if (-not (Test-IsWindows)) {
        throw 'quick-install.ps1 currently supports Windows only.'
    }

    if ($PSVersionTable.PSVersion.Major -lt 7) {
        throw 'quick-install.ps1 requires PowerShell 7 or newer.'
    }

    $options = Parse-Arguments -Arguments $Arguments
    if ($options.Help) {
        Write-Usage
        return
    }

    $tag = Resolve-ReleaseTag -VersionSelector $options.Version
    $assetName = Get-AssetName
    $installDir = if ([string]::IsNullOrWhiteSpace($options.InstallDir)) { Get-DefaultInstallDir } else { $options.InstallDir }
    $resolvedInstallDir = [System.IO.Path]::GetFullPath($installDir)

    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("typo-install-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

    try {
        $binaryPath = Join-Path $tmpDir $assetName
        $checksumsPath = Join-Path $tmpDir 'checksums.txt'

        $releaseBase = Get-ReleaseDownloadBase
        $binaryUrl = Join-Url @($releaseBase, $tag, $assetName)
        $checksumsUrl = Join-Url @($releaseBase, $tag, 'checksums.txt')

        Write-Host "Downloading $assetName from $binaryUrl"
        Download-File -Uri $binaryUrl -OutFile $binaryPath
        Download-File -Uri $checksumsUrl -OutFile $checksumsPath

        $expectedHash = Get-ExpectedChecksum -ChecksumsFile $checksumsPath -AssetName $assetName
        $actualHash = (Get-FileHash -Path $binaryPath -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actualHash -ne $expectedHash) {
            throw "Checksum verification failed for $assetName"
        }

        New-Item -ItemType Directory -Path $resolvedInstallDir -Force | Out-Null
        $installedBinary = Join-Path $resolvedInstallDir "$script:BinaryName.exe"
        Copy-Item -Path $binaryPath -Destination $installedBinary -Force

        $pathUpdate = Ensure-InstallDirInPath -InstallDir $resolvedInstallDir
        Write-NextSteps -InstallDir $resolvedInstallDir -InstalledBinary $installedBinary -PathUpdate $pathUpdate
    } finally {
        if (Test-Path -Path $tmpDir) {
            Remove-Item -Path $tmpDir -Recurse -Force
        }
    }
}

Invoke-TypoQuickInstall -Arguments $args
} @args
