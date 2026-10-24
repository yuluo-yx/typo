package commands

// preSubcommandOptionsWithValues lists global options that appear before a tool's
// subcommand and consume the following token. It is the single source of truth
// for both subcommand detection and long-option scanning in the engine.
var preSubcommandOptionsWithValues = map[string]map[string]bool{
	"git": {
		"-C":             true,
		"-c":             true,
		"--config-env":   true,
		"--exec-path":    true,
		"--git-dir":      true,
		"--namespace":    true,
		"--super-prefix": true,
		"--work-tree":    true,
	},
	"docker": {
		"--config":    true,
		"--context":   true,
		"--host":      true,
		"--log-level": true,
		"-H":          true,
		"-l":          true,
	},
	"go": {
		"-C": true,
	},
	"kubectl": {
		"--as":                    true,
		"--as-group":              true,
		"--as-uid":                true,
		"--cache-dir":             true,
		"--certificate-authority": true,
		"--client-certificate":    true,
		"--client-key":            true,
		"--cluster":               true,
		"--context":               true,
		"--kubeconfig":            true,
		"--namespace":             true,
		"--password":              true,
		"--profile":               true,
		"--request-timeout":       true,
		"--server":                true,
		"--token":                 true,
		"--user":                  true,
		"--username":              true,
		"-n":                      true,
		"-s":                      true,
	},
	"npm": {
		"--cache":      true,
		"--prefix":     true,
		"--userconfig": true,
		"-C":           true,
	},
	"cargo": {
		"--color":   true,
		"--config":  true,
		"--explain": true,
		"-C":        true,
		"-Z":        true,
	},
	"terraform": {
		"-chdir": true,
	},
	"helm": {
		"--burst-limit":       true,
		"--host":              true,
		"--kube-apiserver":    true,
		"--kube-as-group":     true,
		"--kube-as-user":      true,
		"--kube-ca-file":      true,
		"--kube-context":      true,
		"--kube-token":        true,
		"--kubeconfig":        true,
		"--namespace":         true,
		"--qps":               true,
		"--registry-config":   true,
		"--repository-cache":  true,
		"--repository-config": true,
		"-n":                  true,
	},
	"aws": {
		"--ca-bundle":           true,
		"--cli-binary-format":   true,
		"--cli-connect-timeout": true,
		"--cli-read-timeout":    true,
		"--color":               true,
		"--endpoint-url":        true,
		"--output":              true,
		"--profile":             true,
		"--query":               true,
		"--region":              true,
	},
	"gcloud": {
		"--access-token-file":            true,
		"--account":                      true,
		"--billing-project":              true,
		"--configuration":                true,
		"--filter":                       true,
		"--flags-file":                   true,
		"--flatten":                      true,
		"--format":                       true,
		"--impersonate-service-account":  true,
		"--project":                      true,
		"--trace-token":                  true,
		"--user-output-enabled-log-file": true,
		"--verbosity":                    true,
	},
	"az": {
		"--output":       true,
		"--query":        true,
		"--subscription": true,
		"--tenant":       true,
	},
}

// PreSubcommandOptionTakesValue reports whether option, placed before the
// subcommand of tool, consumes the next token as its value.
func PreSubcommandOptionTakesValue(tool, option string) bool {
	return preSubcommandOptionsWithValues[tool][option]
}
