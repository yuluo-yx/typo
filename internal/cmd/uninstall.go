package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/yuluo-yx/typo/internal/config"
)

func cmdUninstall() int {
	fmt.Println("Cleaning up typo...")
	fmt.Println()

	cfg := config.Load()
	hasError := false
	shellName, _ := detectShellIntegrationTarget()

	// Remove ~/.typo directory
	fmt.Print("[1/3] Removing config directory: ")
	if cfg.ConfigDir != "" {
		if err := removeAll(cfg.ConfigDir); err != nil {
			fmt.Printf("✗ failed: %v\n", err)
			hasError = true
		} else {
			fmt.Printf("✓ removed %s\n", cfg.ConfigDir)
		}
	} else {
		fmt.Println("⊘ not found")
	}

	// Show manual cleanup instructions for shell configuration leftovers.
	fmt.Print("[2/3] Shell integration: ")
	homeDir, err := userHomeDir()
	if err != nil {
		fmt.Println("✗ cannot determine home directory")
		hasError = true
	} else {
		foundShellConfig := false
		if shellName == shellNamePowerShell {
			foundShellConfig = true
			fmt.Println("manual cleanup required in $PROFILE.CurrentUserCurrentHost:")
			fmt.Println()
			fmt.Println("    Invoke-Expression (& typo init powershell)")
			fmt.Println()
		}
		for _, target := range []struct {
			shell string
			rc    string
		}{
			{shell: "zsh", rc: ".zshrc"},
			{shell: "bash", rc: ".bashrc"},
			{shell: "fish", rc: ".config/fish/config.fish"},
		} {
			rcPath := filepath.Join(homeDir, target.rc)
			if _, statErr := statPath(rcPath); statErr == nil {
				foundShellConfig = true
				fmt.Printf("manual cleanup required in ~/%s:\n", target.rc)
				fmt.Println()
				if target.shell == "fish" {
					fmt.Println("    typo init fish | source")
				} else {
					fmt.Printf("    eval \"$(typo init %s)\"\n", target.shell)
				}
				fmt.Println()
			}
		}
		if !foundShellConfig {
			fmt.Println("✓ no .zshrc, .bashrc, or .config/fish/config.fish found")
		}
	}

	// Show manual cleanup instructions for the installed binary.
	fmt.Print("[3/3] Binary: ")
	execPath, err := executable()
	if err != nil {
		fmt.Println("✗ cannot determine binary location")
		hasError = true
	} else {
		fmt.Printf("manual cleanup required for the binary:\n")
		fmt.Println()
		fmt.Printf("    rm %s\n", execPath)
		fmt.Println()
	}

	fmt.Println("Local cleanup complete. Manual steps above may still be required.")
	if hasError {
		return 1
	}
	return 0
}
