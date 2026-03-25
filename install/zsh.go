package install

import _ "embed"

// ZshScript is the embedded zsh integration script and the single source of truth for installation.
//
//go:embed typo.zsh
var ZshScript string
