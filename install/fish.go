package install

import _ "embed"

// FishScript is the embedded fish integration script and the single source of truth for installation.
//
//go:embed typo.fish
var FishScript string
