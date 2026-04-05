package install

import _ "embed"

// PowerShellScript is the embedded PowerShell integration script and the single source of truth for installation.
//
//go:embed typo.ps1
var PowerShellScript string
