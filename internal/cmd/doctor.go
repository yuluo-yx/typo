package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuluo-yx/typo/internal/config"
)

func cmdDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOutput := fs.Bool("json", false, "print machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Error: doctor does not accept positional arguments")
		return 1
	}
	if *jsonOutput {
		return cmdDoctorJSON()
	}

	fmt.Println("Checking typo configuration...")
	fmt.Println()

	hasError := false
	cfg := config.Load()
	shellName, shellRC := detectShellIntegrationTarget()

	// Check if typo is in PATH
	fmt.Print("[1/6] typo command: ")
	typoPath, err := lookPath("typo")
	if err == nil {
		fmt.Printf("✓ available in PATH (%s)\n", typoPath)
	} else {
		fmt.Println("✗ not found in PATH")
		// Check if typo exists in Go bin
		goBinPath := checkGoBinTypo()
		if goBinPath != "" {
			fmt.Println()
			fmt.Println("  Found typo in Go bin directory but not in PATH.")
			fmt.Printf("  Add the following to your %s:\n", shellRC)
			fmt.Printf("    export PATH=\"$PATH:%s\"\n", goBinPath)
			fmt.Println()
		}
		hasError = true
	}

	// Check config directory
	fmt.Print("[2/6] config directory: ")
	if info, err := statPath(cfg.ConfigDir); err == nil && info.IsDir() {
		fmt.Printf("✓ %s\n", cfg.ConfigDir)
	} else {
		fmt.Printf("⊘ %s (will be created on first use)\n", cfg.ConfigDir)
	}

	// Check config file and print effective settings
	fmt.Print("[3/6] config file: ")
	if configFile := cfg.ConfigFilePath(); configFile != "" {
		if info, err := statPath(configFile); err == nil && !info.IsDir() {
			fmt.Printf("✓ %s\n", configFile)
		} else {
			fmt.Printf("⊘ %s (using defaults; run 'typo config gen' to create it)\n", configFile)
		}
	} else {
		fmt.Println("⊘ unavailable")
	}

	printDoctorEffectiveConfig(cfg, shellName)
	hasError = checkDoctorShellIntegration(shellName, shellRC) || hasError
	checkDoctorInstallMethod(shellName, shellRC, doctorInstallPath(typoPath))
	hasError = checkDoctorGoBinPath(shellName, shellRC, typoPath) || hasError

	fmt.Println()
	if hasError {
		fmt.Println("Some checks failed. Please fix the issues above.")
		return 1
	}

	fmt.Println("All checks passed!")
	return 0
}

func printDoctorEffectiveConfig(cfg *config.Config, shellName string) {
	fmt.Println()
	fmt.Println("effective config:")
	fmt.Printf("  shell: %s\n", shellName)
	for _, setting := range cfg.ListSettings() {
		fmt.Printf("  %s=%s\n", setting.Key, setting.Value)
	}
	fmt.Println()
}

func checkDoctorShellIntegration(shellName, shellRC string) bool {
	fmt.Print("[4/6] shell integration: ")
	if os.Getenv("TYPO_SHELL_INTEGRATION") == "1" {
		fmt.Println("✓ loaded")
		return printDoctorShellMisconfiguration(shellName)
	}

	fmt.Println("✗ not loaded")
	fmt.Println()
	if shellName != "" {
		fmt.Printf("To enable shell integration, add to your %s:\n", shellRC)
		fmt.Printf("  %s\n", shellInitCommand(shellName))
		fmt.Println()
		fmt.Printf("Then restart your shell or run: %s\n", shellReloadCommand(shellName, shellRC))
		_ = printDoctorShellMisconfiguration(shellName)
		return true
	}

	fmt.Println("To enable shell integration, add one of the following:")
	fmt.Println("  # Zsh (~/.zshrc)")
	fmt.Println("  eval \"$(typo init zsh)\"")
	fmt.Println("  # Bash (~/.bashrc)")
	fmt.Println("  eval \"$(typo init bash)\"")
	fmt.Println("  # Fish (~/.config/fish/config.fish)")
	fmt.Println("  typo init fish | source")
	fmt.Println("  # PowerShell ($PROFILE.CurrentUserCurrentHost)")
	fmt.Printf("  %s\n", shellInitCommand(shellNamePowerShell))
	fmt.Println()
	fmt.Println("Then restart your shell or source the matching profile file.")
	return true
}

type doctorJSONReport struct {
	SchemaVersion int                 `json:"schema_version"`
	OK            bool                `json:"ok"`
	Checks        []doctorJSONCheck   `json:"checks"`
	Shell         doctorJSONShell     `json:"shell"`
	Config        doctorJSONConfig    `json:"config"`
	Install       doctorJSONInstall   `json:"install"`
	GoBinPath     doctorJSONGoBinPath `json:"go_bin_path"`
	Actions       []doctorJSONAction  `json:"actions,omitempty"`
}

type doctorJSONCheck struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Path    string `json:"path,omitempty"`
}

type doctorJSONShell struct {
	Name                        string `json:"name"`
	ConfigFile                  string `json:"config_file"`
	InitCommand                 string `json:"init_command,omitempty"`
	ReloadCommand               string `json:"reload_command,omitempty"`
	IntegrationLoaded           bool   `json:"integration_loaded"`
	Misconfiguration            string `json:"misconfiguration,omitempty"`
	StderrCacheSupported        bool   `json:"stderr_cache_supported"`
	AliasContextSupported       bool   `json:"alias_context_supported"`
	EnvironmentContextSupported bool   `json:"environment_context_supported"`
}

type doctorJSONConfig struct {
	Dir        string                    `json:"dir"`
	DirExists  bool                      `json:"dir_exists"`
	File       string                    `json:"file"`
	FileExists bool                      `json:"file_exists"`
	Settings   []doctorJSONConfigSetting `json:"settings"`
}

type doctorJSONConfigSetting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type doctorJSONInstall struct {
	Method          string `json:"method"`
	Detail          string `json:"detail,omitempty"`
	Path            string `json:"path,omitempty"`
	Action          string `json:"action,omitempty"`
	UpdateSupported bool   `json:"update_supported"`
}

type doctorJSONGoBinPath struct {
	Dir         string `json:"dir,omitempty"`
	TypoInGoBin bool   `json:"typo_in_go_bin"`
	Configured  bool   `json:"configured"`
}

type doctorJSONAction struct {
	ID      string `json:"id"`
	Command string `json:"command"`
}

func cmdDoctorJSON() int {
	report := buildDoctorJSONReport()
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: marshal doctor JSON: %v\n", err)
		return 1
	}
	fmt.Println(string(data))
	if report.OK {
		return 0
	}
	return 1
}

func buildDoctorJSONReport() doctorJSONReport {
	cfg := config.Load()
	shellName, shellRC := detectShellIntegrationTarget()
	typoPath, typoErr := lookPath("typo")
	configFile := cfg.ConfigFilePath()

	report := doctorJSONReport{
		SchemaVersion: 1,
		OK:            true,
		Shell: doctorJSONShell{
			Name:                        shellName,
			ConfigFile:                  shellRC,
			InitCommand:                 shellInitCommand(shellName),
			ReloadCommand:               shellReloadCommand(shellName, shellRC),
			IntegrationLoaded:           os.Getenv("TYPO_SHELL_INTEGRATION") == "1",
			Misconfiguration:            doctorShellMisconfiguration(shellName),
			StderrCacheSupported:        shellSupportsStderrCache(shellName),
			AliasContextSupported:       shellSupportsAliasContext(shellName),
			EnvironmentContextSupported: shellSupportsAliasContext(shellName),
		},
		Config: doctorJSONConfig{
			Dir:      cfg.ConfigDir,
			File:     configFile,
			Settings: doctorJSONConfigSettings(cfg),
		},
	}

	report.addDoctorJSONConfigChecks(cfg, configFile)
	report.addDoctorJSONTypoCheck(typoPath, typoErr)
	report.addDoctorJSONShellIntegrationCheck()
	report.addDoctorJSONInstallCheck(typoPath)
	report.addDoctorJSONGoBinPathCheck(shellName, typoPath)

	return report
}

func doctorJSONConfigSettings(cfg *config.Config) []doctorJSONConfigSetting {
	settings := cfg.ListSettings()
	configSettings := make([]doctorJSONConfigSetting, 0, len(settings))
	for _, setting := range settings {
		configSettings = append(configSettings, doctorJSONConfigSetting{
			Key:   setting.Key,
			Value: setting.Value,
		})
	}
	return configSettings
}

func (r *doctorJSONReport) addDoctorJSONConfigChecks(cfg *config.Config, configFile string) {
	if info, err := statPath(cfg.ConfigDir); err == nil && info.IsDir() {
		r.Config.DirExists = true
		r.addDoctorCheck("config_directory", "config directory", "pass", cfg.ConfigDir, cfg.ConfigDir)
	} else {
		r.addDoctorCheck("config_directory", "config directory", "info", "will be created on first use", cfg.ConfigDir)
	}

	if configFile != "" {
		if info, err := statPath(configFile); err == nil && !info.IsDir() {
			r.Config.FileExists = true
			r.addDoctorCheck("config_file", "config file", "pass", configFile, configFile)
		} else {
			r.addDoctorCheck("config_file", "config file", "info", "using defaults; run 'typo config gen' to create it", configFile)
		}
	} else {
		r.addDoctorCheck("config_file", "config file", "info", "unavailable", "")
	}
}

func (r *doctorJSONReport) addDoctorJSONTypoCheck(typoPath string, typoErr error) {
	if typoErr == nil {
		r.addDoctorCheck("typo_command", "typo command", "pass", "available in PATH", typoPath)
		return
	}

	r.OK = false
	r.addDoctorCheck("typo_command", "typo command", "fail", "not found in PATH", "")
	if goBinPath := checkGoBinTypo(); goBinPath != "" {
		r.Actions = append(r.Actions, doctorJSONAction{
			ID:      "add_go_bin_to_path",
			Command: fmt.Sprintf("export PATH=\"$PATH:%s\"", goBinPath),
		})
	}
}

func (r *doctorJSONReport) addDoctorJSONShellIntegrationCheck() {
	if !r.Shell.IntegrationLoaded {
		r.OK = false
		r.addDoctorCheck("shell_integration", "shell integration", "fail", "not loaded", "")
		if r.Shell.InitCommand != "" {
			r.Actions = append(r.Actions, doctorJSONAction{
				ID:      "enable_shell_integration",
				Command: r.Shell.InitCommand,
			})
		}
		return
	}

	if r.Shell.Misconfiguration != "" {
		r.OK = false
		r.addDoctorCheck("shell_integration", "shell integration", "fail", r.Shell.Misconfiguration, "")
		return
	}

	r.addDoctorCheck("shell_integration", "shell integration", "pass", "loaded", "")
}

func (r *doctorJSONReport) addDoctorJSONInstallCheck(typoPath string) {
	installMethod := detectDoctorInstallMethod(doctorInstallPath(typoPath))
	r.Install = doctorJSONInstall{
		Method:          installMethod.name,
		Detail:          installMethod.detail,
		Path:            installMethod.path,
		Action:          installMethod.action,
		UpdateSupported: installMethod.updateSupported,
	}
	if installMethod.name == UnknownValue {
		r.addDoctorCheck("install_method", "install method", "info", "unable to determine", "")
	} else {
		r.addDoctorCheck("install_method", "install method", "pass", installMethod.name, installMethod.path)
	}
}

func (r *doctorJSONReport) addDoctorJSONGoBinPathCheck(shellName, typoPath string) {
	goBinDir := getGoBinDir()
	r.GoBinPath.Dir = goBinDir
	if typoPath != "" {
		r.GoBinPath.TypoInGoBin = sameDir(filepath.Dir(typoPath), goBinDir)
	}
	if !r.GoBinPath.TypoInGoBin {
		r.addDoctorCheck("go_bin_path", "Go bin PATH", "skip", "not a go install binary", "")
	} else if goBinDir == "" {
		r.addDoctorCheck("go_bin_path", "Go bin PATH", "skip", "Go not installed or GOPATH not set", "")
	} else if pathContainsDir(os.Getenv("PATH"), goBinDir) {
		r.GoBinPath.Configured = true
		r.addDoctorCheck("go_bin_path", "Go bin PATH", "pass", "configured", goBinDir)
	} else {
		r.OK = false
		r.addDoctorCheck("go_bin_path", "Go bin PATH", "fail", "not in PATH", goBinDir)
		r.Actions = append(r.Actions, doctorJSONAction{
			ID:      "add_go_bin_to_path",
			Command: shellPathExportCommand(shellName, goBinDir),
		})
	}
}

func (r *doctorJSONReport) addDoctorCheck(id, name, status, message, path string) {
	r.Checks = append(r.Checks, doctorJSONCheck{
		ID:      id,
		Name:    name,
		Status:  status,
		Message: message,
		Path:    path,
	})
}

func shellSupportsStderrCache(shellName string) bool {
	switch shellName {
	case "zsh", "bash", shellNamePowerShell:
		return true
	default:
		return false
	}
}

func shellSupportsAliasContext(shellName string) bool {
	switch shellName {
	case "zsh", "bash", "fish", shellNamePowerShell:
		return true
	default:
		return false
	}
}

func checkDoctorInstallMethod(shellName, shellRC, typoPath string) {
	method := detectDoctorInstallMethod(typoPath)

	fmt.Print("[5/6] install method: ")
	if method.name == UnknownValue {
		fmt.Println("⊘ unable to determine")
	} else if method.detail != "" {
		fmt.Printf("✓ %s (%s)\n", method.name, method.detail)
	} else {
		fmt.Printf("✓ %s\n", method.name)
	}

	if method.action != "" {
		fmt.Println("  Install/update:")
		fmt.Printf("    %s\n", method.action)
	}
	if method.updateSupported {
		fmt.Println("  typo update: supported")
	} else {
		fmt.Println("  typo update: unsupported")
	}
	if shellName != "" {
		fmt.Printf("  Shell config: %s\n", shellRC)
		fmt.Printf("  Shell init: %s\n", shellInitCommand(shellName))
	} else {
		fmt.Println("  Shell setup: run typo doctor from zsh, bash, fish, or PowerShell for shell-specific instructions.")
	}
}

func checkDoctorGoBinPath(shellName, shellRC, typoPath string) bool {
	fmt.Print("[6/6] Go bin PATH: ")
	goBinDir := getGoBinDir()
	typoInGoBin := false
	if typoPath != "" {
		typoInGoBin = sameDir(filepath.Dir(typoPath), goBinDir)
	}

	if !typoInGoBin {
		fmt.Println("⊘ skipped (not a go install binary)")
		return false
	}
	if goBinDir == "" {
		fmt.Println("⊘ Go not installed or GOPATH not set")
		return false
	}
	if pathContainsDir(os.Getenv("PATH"), goBinDir) {
		fmt.Println("✓ configured")
		return false
	}

	fmt.Printf("✗ %s not in PATH\n", goBinDir)
	fmt.Println()
	if shellName != "" {
		fmt.Printf("  If you installed typo with 'go install', add to your %s:\n", shellRC)
	} else {
		fmt.Println("  If you installed typo with 'go install', add to your shell config:")
	}
	fmt.Printf("    %s\n", shellPathExportCommand(shellName, goBinDir))
	return true
}

func doctorInstallPath(typoPath string) string {
	if typoPath != "" {
		return typoPath
	}

	if execPath, err := executable(); err == nil {
		return execPath
	}
	return ""
}

func printDoctorShellMisconfiguration(shellName string) bool {
	message := doctorShellMisconfiguration(shellName)
	if message == "" {
		return false
	}

	fmt.Println()
	fmt.Println("Common shell integration misconfiguration detected:")
	fmt.Printf("  %s\n", message)
	return true
}

func doctorShellMisconfiguration(shellName string) string {
	configPath := shellConfigFilePath(shellName)
	if configPath == "" {
		return ""
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	content := string(data)
	expected := shellInitCommand(shellName)
	if expected == "" || strings.Contains(content, expected) {
		return ""
	}
	if !strings.Contains(content, "typo init") {
		return ""
	}

	return fmt.Sprintf("%s contains a typo init command that does not match %s. Use: %s", shellConfigDisplayPath(shellName), shellName, expected)
}

// doctorInstallMethod represents detected install method details.
type doctorInstallMethod struct {
	kind            doctorInstallKind
	name            string
	detail          string
	path            string
	action          string
	updateSupported bool
}

type doctorInstallKind int

const (
	doctorInstallUnknown doctorInstallKind = iota
	doctorInstallGo
	doctorInstallWindowsQuick
	doctorInstallHomebrew
	doctorInstallScript
	doctorInstallManual
)

func detectDoctorInstallMethod(typoPath string) doctorInstallMethod {
	if typoPath == "" {
		return doctorInstallMethod{
			kind:   doctorInstallUnknown,
			name:   UnknownValue,
			action: "Install with: curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh | bash",
		}
	}

	if isGoInstallPath(typoPath) {
		return doctorInstallMethod{
			kind:   doctorInstallGo,
			name:   "go install",
			detail: filepath.Dir(filepath.Clean(typoPath)),
			path:   filepath.Clean(typoPath),
			action: "go install github.com/yuluo-yx/typo/cmd/typo@latest",
		}
	}
	if isWindowsQuickInstallPath(typoPath) {
		return doctorInstallMethod{
			kind:   doctorInstallWindowsQuick,
			name:   "Windows quick install",
			detail: filepath.Dir(filepath.Clean(typoPath)),
			path:   filepath.Clean(typoPath),
			action: "iwr -useb https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/quick-install.ps1 | iex",
		}
	}
	if isHomebrewInstallPath(typoPath) {
		return doctorInstallMethod{
			kind:            doctorInstallHomebrew,
			name:            "Homebrew",
			detail:          filepath.Dir(filepath.Clean(typoPath)),
			path:            filepath.Clean(typoPath),
			action:          "brew update && brew upgrade typo",
			updateSupported: true,
		}
	}
	if isScriptInstallPath(typoPath) {
		return doctorInstallMethod{
			kind:            doctorInstallScript,
			name:            "curl/install.sh",
			detail:          filepath.Dir(filepath.Clean(typoPath)),
			path:            filepath.Clean(typoPath),
			action:          "typo update",
			updateSupported: true,
		}
	}

	return doctorInstallMethod{
		kind:   doctorInstallManual,
		name:   "manual release",
		detail: filepath.Dir(filepath.Clean(typoPath)),
		path:   filepath.Clean(typoPath),
		action: "Download the matching GitHub Release binary, verify checksums.txt, then place typo in a PATH directory.",
	}
}

func doctorInstallCandidatePaths(typoPath string) []string {
	candidates := []string{filepath.Clean(typoPath)}
	if resolved, err := filepath.EvalSymlinks(typoPath); err == nil && !sameDir(resolved, typoPath) {
		candidates = append(candidates, filepath.Clean(resolved))
	}
	return candidates
}
