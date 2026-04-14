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
		printIntegrationScript("zsh")
		return 0
	case "bash":
		printIntegrationScript("bash")
		return 0
	case "fish":
		printIntegrationScript("fish")
		return 0
	case shellNamePowerShell:
		printIntegrationScript(shellNamePowerShell)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Unsupported shell: %s\n", args[0])
		return 1
	}
}

func printIntegrationScript(shell string) {
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
		os.Exit(1)
	}

	printScript(script)
}

func printScript(script string) {
	fmt.Print(script)
	if !strings.HasSuffix(script, "\n") {
		fmt.Println()
	}
}
