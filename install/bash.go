package install

import _ "embed"

// BashScript is the embedded bash integration script and the single source of truth for installation.
//
//go:embed typo.bash
var BashScript string
