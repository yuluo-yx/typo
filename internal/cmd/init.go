package cmd

import (
	"fmt"
	"os"
	"strings"
)

func cmdInit(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: shell required (zsh, bash, fish, powershell)")
		return 1
	}

	switch normalizeShellName(args[0]) {
	case "zsh":
		script, code := printIntegrationScript("zsh")
		if code == 0 {
			printScript(script)
		}
		return code
	case "bash":
		script, code := printIntegrationScript("bash")
		if code == 0 {
			printScript(script)
		}
		return code
	case "fish":
		script, code := printIntegrationScript("fish")
		if code == 0 {
			printScript(script)
		}
		return code
	case shellNamePowerShell:
		script, code := printIntegrationScript(shellNamePowerShell)
		if code == 0 {
			printScript(script)
		}
		return code
	default:
		fmt.Fprintf(os.Stderr, "Unsupported shell: %s\n", args[0])
		return 1
	}
}

func printIntegrationScript(shell string) (string, int) {
	var script string
	switch shell {
	case "zsh":
		script = zshIntegrationScript
	case "bash":
		script = bashIntegrationScript
	case "fish":
		script = fishIntegrationScript
	case shellNamePowerShell:
		script = powerShellIntegrationScript
	default:
		fmt.Fprintf(os.Stderr, "Unsupported shell: %s\n", shell)
		return "", 1
	}

	return script, 0
}

func printScript(script string) {
	fmt.Print(script)
	if !strings.HasSuffix(script, "\n") {
		fmt.Println()
	}
}
