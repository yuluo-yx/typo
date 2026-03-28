package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewSubcommandRegistry(t *testing.T) {
	tmpDir := t.TempDir()

	r := NewSubcommandRegistry(tmpDir)
	if r == nil {
		t.Fatal("Expected non-nil registry")
	}
	if r.cacheDir != tmpDir {
		t.Errorf("Expected cacheDir %s, got %s", tmpDir, r.cacheDir)
	}
	if r.cache == nil {
		t.Error("Expected initialized cache map")
	}
	if r.cacheExpiry != 7*24*time.Hour {
		t.Errorf("Expected cacheExpiry 7 days, got %v", r.cacheExpiry)
	}
}

func TestNewSubcommandRegistry_EmptyCacheDir(t *testing.T) {
	r := NewSubcommandRegistry("")
	if r == nil {
		t.Fatal("Expected non-nil registry")
	}
	if r.cacheDir != "" {
		t.Errorf("Expected empty cacheDir, got %s", r.cacheDir)
	}
}

func TestHasBuiltinSubcommand(t *testing.T) {
	if !HasBuiltinSubcommand("git", "status") {
		t.Fatal("Expected builtin git subcommand lookup to find status")
	}
	if HasBuiltinSubcommand("git", "nonexistent-subcommand") {
		t.Fatal("Expected unknown git subcommand lookup to fail")
	}
}

func TestLoadCache(t *testing.T) {
	t.Run("loads existing cache", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheFile := filepath.Join(tmpDir, "subcommands.json")

		caches := []SubcommandCache{
			{Tool: "git", Subcommands: []string{"add", "commit"}, UpdatedAt: time.Now()},
			{Tool: "docker", Subcommands: []string{"build", "run"}, UpdatedAt: time.Now()},
		}
		data, _ := json.MarshalIndent(caches, "", "  ")
		if err := os.WriteFile(cacheFile, data, 0644); err != nil {
			t.Fatalf("Failed to write cache file: %v", err)
		}

		r := &SubcommandRegistry{
			cache:    make(map[string]*SubcommandCache),
			cacheDir: tmpDir,
		}
		r.loadCache()

		if len(r.cache) != 2 {
			t.Errorf("Expected 2 cached tools, got %d", len(r.cache))
		}
		if r.cache["git"] == nil || len(r.cache["git"].Subcommands) != 2 {
			t.Error("Expected git cache with 2 subcommands")
		}
	})

	t.Run("handles nonexistent cache file", func(t *testing.T) {
		r := &SubcommandRegistry{
			cache:    make(map[string]*SubcommandCache),
			cacheDir: t.TempDir(),
		}
		r.loadCache()

		if len(r.cache) != 0 {
			t.Errorf("Expected empty cache, got %d", len(r.cache))
		}
	})

	t.Run("handles invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheFile := filepath.Join(tmpDir, "subcommands.json")
		if err := os.WriteFile(cacheFile, []byte("invalid json"), 0644); err != nil {
			t.Fatalf("Failed to write cache file: %v", err)
		}

		r := &SubcommandRegistry{
			cache:    make(map[string]*SubcommandCache),
			cacheDir: tmpDir,
		}
		r.loadCache()

		if len(r.cache) != 0 {
			t.Fatalf("Expected invalid JSON load to keep explicit cache empty, got %d", len(r.cache))
		}

		matches, err := filepath.Glob(cacheFile + ".corrupt-*")
		if err != nil {
			t.Fatalf("Glob failed: %v", err)
		}
		if len(matches) != 1 {
			t.Fatalf("Expected one quarantined cache file, got %v", matches)
		}
	})

	t.Run("handles empty cacheDir", func(t *testing.T) {
		r := &SubcommandRegistry{
			cache:    make(map[string]*SubcommandCache),
			cacheDir: "",
		}
		r.loadCache()

		if len(r.cache) != 0 {
			t.Errorf("Expected empty cache, got %d", len(r.cache))
		}
	})
}

func TestSaveCache(t *testing.T) {
	t.Run("saves cache to file", func(t *testing.T) {
		tmpDir := t.TempDir()
		r := &SubcommandRegistry{
			cache: map[string]*SubcommandCache{
				"git":    {Tool: "git", Subcommands: []string{"add", "commit"}, UpdatedAt: time.Now()},
				"docker": {Tool: "docker", Subcommands: []string{"build", "run"}, UpdatedAt: time.Now()},
			},
			cacheDir: tmpDir,
		}

		r.saveCache()

		cacheFile := filepath.Join(tmpDir, "subcommands.json")
		data, err := os.ReadFile(cacheFile)
		if err != nil {
			t.Fatalf("Failed to read cache file: %v", err)
		}

		var caches []SubcommandCache
		if err := json.Unmarshal(data, &caches); err != nil {
			t.Fatalf("Failed to unmarshal cache: %v", err)
		}

		if len(caches) != 2 {
			t.Errorf("Expected 2 cached tools, got %d", len(caches))
		}
	})

	t.Run("handles empty cacheDir", func(t *testing.T) {
		r := &SubcommandRegistry{
			cache: map[string]*SubcommandCache{
				"git": {Tool: "git", Subcommands: []string{"add"}, UpdatedAt: time.Now()},
			},
			cacheDir: "",
		}

		r.saveCache()
	})
}

func TestGet_Cached(t *testing.T) {
	tmpDir := t.TempDir()
	r := &SubcommandRegistry{
		cache: map[string]*SubcommandCache{
			"git": {
				Tool:        "git",
				Subcommands: []string{"add", "commit", "push"},
				UpdatedAt:   time.Now(),
			},
		},
		cacheDir:    tmpDir,
		cacheExpiry: 7 * 24 * time.Hour,
	}

	subcommands := r.Get("git")
	if len(subcommands) != 3 {
		t.Errorf("Expected 3 subcommands, got %d", len(subcommands))
	}
}

func TestGet_ExpiredCache(t *testing.T) {
	tmpDir := t.TempDir()
	r := &SubcommandRegistry{
		cache: map[string]*SubcommandCache{
			"git": {
				Tool:        "git",
				Subcommands: []string{"add", "commit"},
				UpdatedAt:   time.Now().Add(-8 * 24 * time.Hour),
			},
		},
		cacheDir:    tmpDir,
		cacheExpiry: 7 * 24 * time.Hour,
	}

	subcommands := r.Get("git")
	_ = subcommands
}

func TestGet_NonexistentTool(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewSubcommandRegistry(tmpDir)

	subcommands := r.Get("nonexistenttool12345")
	if subcommands != nil {
		t.Errorf("Expected nil for nonexistent tool, got %v", subcommands)
	}
}

func TestFetchSubcommands_NonexistentTool(t *testing.T) {
	r := &SubcommandRegistry{
		cache:       make(map[string]*SubcommandCache),
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
	}

	subcommands := r.fetchSubcommands("nonexistenttool12345")
	if subcommands != nil {
		t.Errorf("Expected nil for nonexistent tool, got %v", subcommands)
	}
}

func TestParseGitHelp(t *testing.T) {
	output := `usage: git [--version] [--help] [-C <path>] [-c <name>=<value>]

Commands
  add                  Add file contents to the index
  commit               Record changes to the repository
  rev-parse            Pick out and massage parameters
  push                 Update remote refs
  ls-files             Show information about files in the index
`

	subcommands := parseGitHelp(output)
	expected := []string{"add", "commit", "rev-parse", "push", "ls-files"}

	if len(subcommands) != len(expected) {
		t.Errorf("Expected %d subcommands, got %d", len(expected), len(subcommands))
	}

	for i, cmd := range expected {
		if i >= len(subcommands) || subcommands[i] != cmd {
			t.Errorf("Expected subcommands[%d] = %s, got %s", i, cmd, subcommands[i])
		}
	}
}

func TestParseGitHelp_Empty(t *testing.T) {
	subcommands := parseGitHelp("")
	if len(subcommands) != 0 {
		t.Errorf("Expected 0 subcommands for empty output, got %d", len(subcommands))
	}
}

func TestParseDockerHelp(t *testing.T) {
	output := `Usage:  docker [OPTIONS] COMMAND

Commands:
  builder     Manage builds
  build       Build an image from a Dockerfile
  run         Run a container

`

	subcommands := parseDockerHelp(output)
	expected := []string{"builder", "build", "run"}

	if len(subcommands) != len(expected) {
		t.Errorf("Expected %d subcommands, got %d: %v", len(expected), len(subcommands), subcommands)
	}

	for i, cmd := range expected {
		if i >= len(subcommands) || subcommands[i] != cmd {
			t.Errorf("Expected subcommands[%d] = %s, got %s", i, cmd, subcommands[i])
		}
	}
}

func TestParseDockerHelp_ManagementCommands(t *testing.T) {
	output := `Usage:  docker [OPTIONS] COMMAND

Management Commands:
  container   Manage containers
  image       Manage images
`

	subcommands := parseDockerHelp(output)
	expected := []string{"container", "image"}

	if len(subcommands) != len(expected) {
		t.Errorf("Expected %d subcommands, got %d", len(expected), len(subcommands))
	}

	for i, cmd := range expected {
		if i >= len(subcommands) || subcommands[i] != cmd {
			t.Errorf("Expected subcommands[%d] = %s, got %s", i, cmd, subcommands[i])
		}
	}
}

func TestParseDockerHelp_Empty(t *testing.T) {
	subcommands := parseDockerHelp("")
	if len(subcommands) != 0 {
		t.Errorf("Expected 0 subcommands for empty output, got %d", len(subcommands))
	}
}

func TestParseNpmHelp(t *testing.T) {
	output := `Usage: npm <command>

where <command> is one of:
  dist-tag, disttag    Manage distribution tags
  install, i, add      Install a package
  run, run-script      Run arbitrary package scripts
  test, t              Run tests
`

	subcommands := parseNpmHelp(output)

	for _, cmd := range []string{"dist-tag", "install", "run", "test"} {
		found := false
		for _, s := range subcommands {
			if s == cmd {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find %s in subcommands", cmd)
		}
	}
}

func TestParseNpmHelp_Empty(t *testing.T) {
	subcommands := parseNpmHelp("")
	if len(subcommands) != 0 {
		t.Errorf("Expected 0 subcommands for empty output, got %d", len(subcommands))
	}
}

func TestParseYarnHelp(t *testing.T) {
	output := `Usage: yarn <command>

Commands:
  add       Installs a package and any packages that it depends on.
  init      Interactively creates or updates a package.json file.
  install   Installs all dependencies.
`

	subcommands := parseYarnHelp(output)
	expected := []string{"add", "init", "install"}

	if len(subcommands) != len(expected) {
		t.Errorf("Expected %d subcommands, got %d", len(expected), len(subcommands))
	}

	for i, cmd := range expected {
		if i >= len(subcommands) || subcommands[i] != cmd {
			t.Errorf("Expected subcommands[%d] = %s, got %s", i, cmd, subcommands[i])
		}
	}
}

func TestParseYarnHelp_Empty(t *testing.T) {
	subcommands := parseYarnHelp("")
	if len(subcommands) != 0 {
		t.Errorf("Expected 0 subcommands for empty output, got %d", len(subcommands))
	}
}

func TestParseKubectlHelp(t *testing.T) {
	output := `kubectl controls the Kubernetes cluster manager.

Basic Commands:
  get           Display one or many resources
  describe      Show details of a specific resource
  delete        Delete resources
`

	subcommands := parseKubectlHelp(output)
	expected := []string{"get", "describe", "delete"}

	if len(subcommands) != len(expected) {
		t.Errorf("Expected %d subcommands, got %d", len(expected), len(subcommands))
	}

	for i, cmd := range expected {
		if i >= len(subcommands) || subcommands[i] != cmd {
			t.Errorf("Expected subcommands[%d] = %s, got %s", i, cmd, subcommands[i])
		}
	}
}

func TestParseKubectlHelp_Empty(t *testing.T) {
	subcommands := parseKubectlHelp("")
	if len(subcommands) != 0 {
		t.Errorf("Expected 0 subcommands for empty output, got %d", len(subcommands))
	}
}

func TestParseCargoHelp(t *testing.T) {
	output := `Usage: cargo <command>

Commands:
    build, b    Compile the current package
    check, c    Analyze the current package
    run, r      Run the current package
`

	subcommands := parseCargoHelp(output)

	for _, cmd := range []string{"build", "check", "run"} {
		found := false
		for _, s := range subcommands {
			if s == cmd {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find %s in subcommands", cmd)
		}
	}
}

func TestParseCargoHelp_Empty(t *testing.T) {
	subcommands := parseCargoHelp("")
	if len(subcommands) != 0 {
		t.Errorf("Expected 0 subcommands for empty output, got %d", len(subcommands))
	}
}

func TestParseGoHelp(t *testing.T) {
	output := `Go is a tool for managing Go source code.

Usage:

	go <command> [arguments]

The commands are:

	build       compile packages and dependencies
	clean       remove object files and cached files
	test        test packages

Use "go help <command>" for more information about a command.

Additional help topics:

	buildconstraint build constraints
`

	subcommands := parseGoHelp(output)
	expected := []string{"build", "clean", "test"}

	if len(subcommands) != len(expected) {
		t.Errorf("Expected %d subcommands, got %d: %v", len(expected), len(subcommands), subcommands)
		return
	}

	for i, cmd := range expected {
		if subcommands[i] != cmd {
			t.Errorf("Expected subcommands[%d] = %s, got %s", i, cmd, subcommands[i])
		}
	}
}

func TestParseGoHelp_Empty(t *testing.T) {
	subcommands := parseGoHelp("")
	if len(subcommands) != 0 {
		t.Errorf("Expected 0 subcommands for empty output, got %d", len(subcommands))
	}
}

func TestParseBrewHelp(t *testing.T) {
	output := `==> Built-in commands
--cache
--repository
install
upgrade
search
`

	subcommands := parseBrewHelp(output)
	expected := []string{"install", "upgrade", "search"}

	if len(subcommands) != len(expected) {
		t.Fatalf("Expected %d subcommands, got %d: %v", len(expected), len(subcommands), subcommands)
	}

	for i, cmd := range expected {
		if subcommands[i] != cmd {
			t.Fatalf("Expected subcommands[%d] = %s, got %s", i, cmd, subcommands[i])
		}
	}
}

func TestParseGenericHelp(t *testing.T) {
	output := `Usage: mytool <command>

Commands:
  start    Start the service
  stop     Stop the service
  status   Show service status
`

	subcommands := parseGenericHelp(output)
	expected := []string{"start", "stop", "status"}

	if len(subcommands) != len(expected) {
		t.Errorf("Expected %d subcommands, got %d", len(expected), len(subcommands))
	}

	for i, cmd := range expected {
		if i >= len(subcommands) || subcommands[i] != cmd {
			t.Errorf("Expected subcommands[%d] = %s, got %s", i, cmd, subcommands[i])
		}
	}
}

func TestParseGenericHelp_Empty(t *testing.T) {
	subcommands := parseGenericHelp("")
	if len(subcommands) != 0 {
		t.Errorf("Expected 0 subcommands for empty output, got %d", len(subcommands))
	}
}

func TestHasSubcommands(t *testing.T) {
	r := &SubcommandRegistry{}

	tests := []struct {
		tool     string
		expected bool
	}{
		{"git", true},
		{"docker", true},
		{"npm", true},
		{"yarn", true},
		{"kubectl", true},
		{"cargo", true},
		{"go", true},
		{"pip", true},
		{"pip3", true},
		{"composer", true},
		{"ansible", true},
		{"terraform", true},
		{"helm", true},
		{"brew", true},
		{"unknown", false},
		{"ls", false},
		{"", false},
	}

	for _, tt := range tests {
		result := r.HasSubcommands(tt.tool)
		if result != tt.expected {
			t.Errorf("HasSubcommands(%q) = %v, expected %v", tt.tool, result, tt.expected)
		}
	}
}

func TestPreFetch(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewSubcommandRegistry(tmpDir)

	r.PreFetch()
}

func TestFetchSubcommands_ToolNotInPath(t *testing.T) {
	r := &SubcommandRegistry{
		cache:       make(map[string]*SubcommandCache),
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
	}

	result := r.fetchSubcommands("nonexistentcommand12345xyz")
	if result != nil {
		t.Errorf("Expected nil for tool not in PATH, got %v", result)
	}
}

func TestFetchSubcommands_DefaultParser(t *testing.T) {
	r := &SubcommandRegistry{
		cache:       make(map[string]*SubcommandCache),
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
	}

	result := r.fetchSubcommands("ls")
	_ = result
}

func TestGet_WithFetch(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewSubcommandRegistry(tmpDir)

	_ = r.Get("ls")
}

func TestGet_CacheExpiredAndRefetch(t *testing.T) {
	tmpDir := t.TempDir()
	r := &SubcommandRegistry{
		cache: map[string]*SubcommandCache{
			"ls": {
				Tool:        "ls",
				Subcommands: []string{"old"},
				UpdatedAt:   time.Now().Add(-10 * 24 * time.Hour),
			},
		},
		cacheDir:    tmpDir,
		cacheExpiry: 7 * 24 * time.Hour,
	}

	result := r.Get("ls")
	_ = result
}

func TestSaveCache_CreateDir(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "nested", "cache")

	r := &SubcommandRegistry{
		cache: map[string]*SubcommandCache{
			"git": {Tool: "git", Subcommands: []string{"add"}, UpdatedAt: time.Now()},
		},
		cacheDir: cacheDir,
	}

	r.saveCache()

	if _, err := os.Stat(filepath.Join(cacheDir, "subcommands.json")); os.IsNotExist(err) {
		t.Error("Expected cache file to be created")
	}
}

func TestFetchSubcommands_Git(t *testing.T) {
	if GetPath("git") == "" {
		t.Skip("git not found in PATH")
	}
	r := &SubcommandRegistry{
		cache:       make(map[string]*SubcommandCache),
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
	}
	result := r.fetchSubcommands("git")
	if len(result) == 0 {
		t.Error("Expected git subcommands")
	}
}

func TestFetchSubcommands_Docker(t *testing.T) {
	if GetPath("docker") == "" {
		t.Skip("docker not found in PATH")
	}
	r := &SubcommandRegistry{
		cache:       make(map[string]*SubcommandCache),
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
	}
	result := r.fetchSubcommands("docker")
	_ = result
}

func TestFetchSubcommands_Npm(t *testing.T) {
	if GetPath("npm") == "" {
		t.Skip("npm not found in PATH")
	}
	r := &SubcommandRegistry{
		cache:       make(map[string]*SubcommandCache),
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
	}
	result := r.fetchSubcommands("npm")
	_ = result
}

func TestFetchSubcommands_Go(t *testing.T) {
	if GetPath("go") == "" {
		t.Skip("go not found in PATH")
	}
	r := &SubcommandRegistry{
		cache:       make(map[string]*SubcommandCache),
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
	}
	result := r.fetchSubcommands("go")
	_ = result
}

func TestFetchSubcommands_Kubectl(t *testing.T) {
	if GetPath("kubectl") == "" {
		t.Skip("kubectl not found in PATH")
	}
	r := &SubcommandRegistry{
		cache:       make(map[string]*SubcommandCache),
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
	}
	result := r.fetchSubcommands("kubectl")
	_ = result
}

func TestFetchSubcommands_Cargo(t *testing.T) {
	if GetPath("cargo") == "" {
		t.Skip("cargo not found in PATH")
	}
	r := &SubcommandRegistry{
		cache:       make(map[string]*SubcommandCache),
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
	}
	result := r.fetchSubcommands("cargo")
	_ = result
}

func TestFetchSubcommands_Yarn(t *testing.T) {
	if GetPath("yarn") == "" {
		t.Skip("yarn not found in PATH")
	}
	r := &SubcommandRegistry{
		cache:       make(map[string]*SubcommandCache),
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
	}
	result := r.fetchSubcommands("yarn")
	_ = result
}

func TestGetHelpOutput_GitPrefersHelpA(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	defer os.Setenv("PATH", oldPath)

	gitPath := filepath.Join(tmpDir, "git")
	script := "#!/bin/sh\nif [ \"$1\" = \"help\" ] && [ \"$2\" = \"-a\" ]; then\n  echo git-help-a\n  exit 0\nfi\nif [ \"$1\" = \"--help\" ]; then\n  echo git-help\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(gitPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write git stub: %v", err)
	}

	os.Setenv("PATH", tmpDir)
	r := &SubcommandRegistry{}
	output, err := r.getHelpOutput("git")
	if err != nil {
		t.Fatalf("getHelpOutput failed: %v", err)
	}
	if strings.TrimSpace(output) != "git-help-a" {
		t.Fatalf("Expected help -a output, got %q", output)
	}
}

func TestGetHelpOutput_FallsBackToHelpSubcommand(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	defer os.Setenv("PATH", oldPath)

	toolPath := filepath.Join(tmpDir, "mytool")
	script := "#!/bin/sh\nif [ \"$1\" = \"--help\" ]; then\n  exit 1\nfi\nif [ \"$1\" = \"help\" ]; then\n  echo fallback-help\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(toolPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write tool stub: %v", err)
	}

	os.Setenv("PATH", tmpDir)
	r := &SubcommandRegistry{}
	output, err := r.getHelpOutput("mytool")
	if err != nil {
		t.Fatalf("getHelpOutput failed: %v", err)
	}
	if strings.TrimSpace(output) != "fallback-help" {
		t.Fatalf("Expected help fallback output, got %q", output)
	}
}

func TestGetHelpOutput_BrewUsesCommands(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	defer os.Setenv("PATH", oldPath)

	brewPath := filepath.Join(tmpDir, "brew")
	script := "#!/bin/sh\nif [ \"$1\" = \"commands\" ]; then\n  echo install\n  echo upgrade\n  exit 0\nfi\nif [ \"$1\" = \"--help\" ]; then\n  echo should-not-be-used\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(brewPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write brew stub: %v", err)
	}

	os.Setenv("PATH", tmpDir)
	r := &SubcommandRegistry{}
	output, err := r.getHelpOutput("brew")
	if err != nil {
		t.Fatalf("getHelpOutput failed: %v", err)
	}
	if strings.Contains(output, "should-not-be-used") || !strings.Contains(output, "install") {
		t.Fatalf("Expected brew commands output, got %q", output)
	}
}

func TestFetchSubcommands_DefaultParserWithStub(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	defer os.Setenv("PATH", oldPath)

	toolPath := filepath.Join(tmpDir, "mytool")
	script := "#!/bin/sh\nif [ \"$1\" = \"--help\" ]; then\n  printf 'Usage: mytool <command>\\n\\n  start    Start service\\n  stop     Stop service\\n'\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(toolPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write tool stub: %v", err)
	}

	os.Setenv("PATH", tmpDir)
	r := &SubcommandRegistry{
		cache:       make(map[string]*SubcommandCache),
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
	}

	subcommands := r.fetchSubcommands("mytool")
	if len(subcommands) != 2 || subcommands[0] != "start" || subcommands[1] != "stop" {
		t.Fatalf("Expected generic parser subcommands, got %v", subcommands)
	}
}
