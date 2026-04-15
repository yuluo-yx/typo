package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewToolTreeRegistry(t *testing.T) {
	tmpDir := t.TempDir()

	r := NewToolTreeRegistry(tmpDir)
	if r == nil {
		t.Fatal("Expected non-nil registry")
	}
	if r.cacheDir != tmpDir {
		t.Errorf("Expected cacheDir %s, got %s", tmpDir, r.cacheDir)
	}
	if r.trees == nil {
		t.Error("Expected initialized tree map")
	}
	if r.cacheExpiry != 7*24*time.Hour {
		t.Errorf("Expected cacheExpiry 7 days, got %v", r.cacheExpiry)
	}
}

func TestNewToolTreeRegistry_EmptyCacheDir(t *testing.T) {
	r := NewToolTreeRegistry("")
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
	if !HasBuiltinSubcommand("docker", "image") {
		t.Fatal("Expected builtin docker subcommand lookup to find image")
	}
	if HasBuiltinSubcommand("pip", "install") {
		t.Fatal("Expected dynamic-only pip lookup to have no builtin fallback")
	}
	if HasBuiltinSubcommand("git", "nonexistent-subcommand") {
		t.Fatal("Expected unknown git subcommand lookup to fail")
	}
	if HasBuiltinSubcommand("", "status") {
		t.Fatal("Expected empty tool lookup to fail")
	}
	if HasBuiltinSubcommand("git", "") {
		t.Fatal("Expected empty subcommand lookup to fail")
	}
}

func TestToolTreeRegistry_GetMergesBuiltinSubcommands(t *testing.T) {
	r := &ToolTreeRegistry{
		trees: map[string]*ToolTreeCache{
			"cargo": {
				Tool: "cargo",
				Tree: treeBranch(map[string]*TreeNode{
					"build": {},
					"check": {},
				}),
				UpdatedAt: time.Now(),
			},
		},
		cacheExpiry: 7 * 24 * time.Hour,
	}

	subcommands := r.Get("cargo")
	hasHelp := false
	for _, subcommand := range subcommands {
		if subcommand == "help" {
			hasHelp = true
			break
		}
	}
	if !hasHelp {
		t.Fatalf("Expected builtin cargo subcommands to include help, got %v", subcommands)
	}
}

func TestLoadCache_LoadsExistingCache(t *testing.T) {
	tmpDir := t.TempDir()
	cacheFile := filepath.Join(tmpDir, "subcommands.json")

	wrapper := struct {
		SchemaVersion int              `json:"schema_version"`
		Tools         []*ToolTreeCache `json:"tools"`
	}{
		SchemaVersion: subcommandCacheSchemaVersion,
		Tools: []*ToolTreeCache{
			{
				Tool: "git",
				Tree: treeBranch(map[string]*TreeNode{
					"add":    {},
					"commit": {},
				}),
				UpdatedAt: time.Now(),
			},
			{
				Tool: "gcloud",
				Tree: treeBranch(map[string]*TreeNode{
					"auth": {},
					"compute": treeBranch(map[string]*TreeNode{
						"instances": {},
					}),
				}),
				UpdatedAt: time.Now(),
			},
		},
	}
	data, _ := json.MarshalIndent(wrapper, "", "  ")
	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	r := &ToolTreeRegistry{
		trees:    make(map[string]*ToolTreeCache),
		cacheDir: tmpDir,
	}
	r.loadCache()

	if len(r.trees) != 2 {
		t.Errorf("Expected 2 cached tools, got %d", len(r.trees))
	}
	if got := r.Get("git"); !hasString(got, "add") || !hasString(got, "commit") {
		t.Error("Expected git cache with 2 subcommands")
	}
	if got := r.GetChildren("gcloud", []string{"compute"}); len(got) != 1 || got[0] != "instances" {
		t.Error("Expected gcloud cache with nested children")
	}
}

func TestLoadCache_NonexistentCacheFile(t *testing.T) {
	r := &ToolTreeRegistry{
		trees:    make(map[string]*ToolTreeCache),
		cacheDir: t.TempDir(),
	}
	r.loadCache()

	if len(r.trees) != 0 {
		t.Errorf("Expected empty cache, got %d", len(r.trees))
	}
}

func TestLoadCache_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cacheFile := filepath.Join(tmpDir, "subcommands.json")
	if err := os.WriteFile(cacheFile, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	r := &ToolTreeRegistry{
		trees:    make(map[string]*ToolTreeCache),
		cacheDir: tmpDir,
	}
	r.loadCache()

	if len(r.trees) != 0 {
		t.Fatalf("Expected invalid JSON load to keep explicit cache empty, got %d", len(r.trees))
	}

	matches, err := filepath.Glob(cacheFile + ".corrupt-*")
	if err != nil {
		t.Fatalf("Glob failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("Expected one quarantined cache file, got %v", matches)
	}
}

func TestLoadCache_EmptyCacheDir(t *testing.T) {
	r := &ToolTreeRegistry{
		trees:    make(map[string]*ToolTreeCache),
		cacheDir: "",
	}
	r.loadCache()

	if len(r.trees) != 0 {
		t.Errorf("Expected empty cache, got %d", len(r.trees))
	}
}

func TestLoadCache_LegacyArrayCache(t *testing.T) {
	tmpDir := t.TempDir()
	cacheFile := filepath.Join(tmpDir, "subcommands.json")
	if err := os.WriteFile(cacheFile, []byte(`[{"tool":"git","subcommands":["status"]}]`), 0644); err != nil {
		t.Fatalf("Failed to write legacy cache file: %v", err)
	}

	r := &ToolTreeRegistry{
		trees:    make(map[string]*ToolTreeCache),
		cacheDir: tmpDir,
	}
	r.loadCache()

	if len(r.trees) != 0 {
		t.Fatalf("Expected legacy cache load to keep tree cache empty, got %d", len(r.trees))
	}
	matches, err := filepath.Glob(cacheFile + ".corrupt-*")
	if err != nil {
		t.Fatalf("Glob failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("Expected one quarantined legacy cache file, got %v", matches)
	}
}

func hasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestSaveCache(t *testing.T) {
	t.Run("saves cache to file", func(t *testing.T) {
		tmpDir := t.TempDir()
		r := &ToolTreeRegistry{
			trees: map[string]*ToolTreeCache{
				"git": {
					Tool: "git",
					Tree: treeBranch(map[string]*TreeNode{
						"add":    {},
						"commit": {},
					}),
					UpdatedAt: time.Now(),
				},
				"docker": {
					Tool: "docker",
					Tree: treeBranch(map[string]*TreeNode{
						"build": {},
						"run":   {},
					}),
					UpdatedAt: time.Now(),
				},
			},
			cacheDir: tmpDir,
		}

		r.saveCache()

		cacheFile := filepath.Join(tmpDir, "subcommands.json")
		data, err := os.ReadFile(cacheFile)
		if err != nil {
			t.Fatalf("Failed to read cache file: %v", err)
		}

		var wrapper struct {
			SchemaVersion int              `json:"schema_version"`
			Tools         []*ToolTreeCache `json:"tools"`
		}
		if err := json.Unmarshal(data, &wrapper); err != nil {
			t.Fatalf("Failed to unmarshal cache: %v", err)
		}

		if wrapper.SchemaVersion != subcommandCacheSchemaVersion {
			t.Fatalf("Expected schema version %d, got %d", subcommandCacheSchemaVersion, wrapper.SchemaVersion)
		}
		if len(wrapper.Tools) != 2 {
			t.Errorf("Expected 2 cached tools, got %d", len(wrapper.Tools))
		}
	})

	t.Run("handles empty cacheDir", func(t *testing.T) {
		r := &ToolTreeRegistry{
			trees: map[string]*ToolTreeCache{
				"git": {
					Tool:      "git",
					Tree:      treeBranch(map[string]*TreeNode{"add": {}}),
					UpdatedAt: time.Now(),
				},
			},
			cacheDir: "",
		}

		r.saveCache()
	})
}

func TestGet_Cached(t *testing.T) {
	tmpDir := t.TempDir()
	r := &ToolTreeRegistry{
		trees: map[string]*ToolTreeCache{
			"git": {
				Tool: "git",
				Tree: treeBranch(map[string]*TreeNode{
					"add":    {},
					"commit": {},
					"push":   {},
				}),
				UpdatedAt: time.Now(),
			},
		},
		cacheDir:    tmpDir,
		cacheExpiry: 7 * 24 * time.Hour,
	}

	subcommands := r.Get("git")
	for _, want := range []string{"add", "commit", "push"} {
		found := false
		for _, subcommand := range subcommands {
			if subcommand == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Expected cached subcommands to include %q, got %v", want, subcommands)
		}
	}
}

func TestGetChildren_CachedNested(t *testing.T) {
	r := &ToolTreeRegistry{
		trees: map[string]*ToolTreeCache{
			"gcloud": {
				Tool: "gcloud",
				Tree: treeBranch(map[string]*TreeNode{
					"compute": treeBranch(map[string]*TreeNode{
						"instances": treeBranch(map[string]*TreeNode{
							"describe": {},
							"list":     {},
						}),
					}),
				}),
				UpdatedAt: time.Now(),
			},
		},
		cacheExpiry: 7 * 24 * time.Hour,
	}

	children := r.GetChildren("gcloud", []string{"compute"})
	if len(children) != 1 || children[0] != "instances" {
		t.Fatalf("Expected cached nested children, got %v", children)
	}

	grandChildren := r.GetChildren("gcloud", []string{"compute", "instances"})
	if len(grandChildren) != 2 || !hasString(grandChildren, "list") || !hasString(grandChildren, "describe") {
		t.Fatalf("Expected cached grand-children, got %v", grandChildren)
	}
}

func TestToolTreeRegistry_BuiltinRootCoverageForCommonTools(t *testing.T) {
	r := &ToolTreeRegistry{cacheExpiry: 7 * 24 * time.Hour}

	tests := []struct {
		tool string
		want []string
	}{
		{tool: "npm", want: []string{"install", "run", "test", "ci"}},
		{tool: "yarn", want: []string{"add", "install", "run", "test"}},
		{tool: "cargo", want: []string{"build", "check", "fmt", "test"}},
		{tool: "go", want: []string{"build", "mod", "test", "tool"}},
		{tool: "brew", want: []string{"install", "search", "update", "upgrade"}},
		{tool: "terraform", want: []string{"init", "plan", "apply", "validate"}},
		{tool: "helm", want: []string{"install", "repo", "template", "upgrade"}},
		{tool: "aws", want: []string{"s3", "ec2", "lambda", "sts"}},
		{tool: "gcloud", want: []string{"compute", "config", "storage"}},
		{tool: "az", want: []string{"account", "group", "storage", "vm"}},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got := r.GetChildren(tt.tool, nil)
			for _, want := range tt.want {
				if !hasString(got, want) {
					t.Fatalf("GetChildren(%q) missing %q in %v", tt.tool, want, got)
				}
			}
		})
	}
}

func TestToolTreeRegistry_BuiltinNestedSemantics(t *testing.T) {
	r := &ToolTreeRegistry{cacheExpiry: 7 * 24 * time.Hour}

	tests := []struct {
		name   string
		tool   string
		prefix []string
		want   []string
	}{
		{name: "git stash", tool: "git", prefix: []string{"stash"}, want: []string{"save", "list", "pop", "clear"}},
		{name: "docker container", tool: "docker", prefix: []string{"container"}, want: []string{"start", "stop", "logs", "exec"}},
		{name: "docker image", tool: "docker", prefix: []string{"image"}, want: []string{"build", "list", "ls", "pull", "push"}},
		{name: "kubectl get resources", tool: "kubectl", prefix: []string{"get"}, want: []string{"pods", "po", "deployments", "svc", "namespaces", "ns"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.GetChildren(tt.tool, tt.prefix)
			for _, want := range tt.want {
				if !hasString(got, want) {
					t.Fatalf("GetChildren(%q, %v) missing %q in %v", tt.tool, tt.prefix, want, got)
				}
			}
		})
	}

	if canonical, ok := r.ResolveChild("kubectl", []string{"get"}, "po"); !ok || canonical != "pods" {
		t.Fatalf("Expected kubectl get po to resolve to pods, got %q ok=%v", canonical, ok)
	}
	if canonical, ok := r.ResolveChild("docker", []string{"image"}, "list"); !ok || canonical != "list" {
		t.Fatalf("Expected docker image list to stay valid, got %q ok=%v", canonical, ok)
	}
	if got := r.GetChildren("git", []string{"commit"}); len(got) != 0 {
		t.Fatalf("Expected passthrough git commit to have no child candidates, got %v", got)
	}
}

func TestToolTreeRegistry_CachedNestedCoverageForAdditionalTools(t *testing.T) {
	r := &ToolTreeRegistry{
		trees: map[string]*ToolTreeCache{
			"pip": {
				Tool: "pip",
				Tree: treeBranch(map[string]*TreeNode{
					"cache": treeBranch(map[string]*TreeNode{
						"dir":    {},
						"info":   {},
						"list":   {},
						"purge":  {},
						"remove": {},
					}),
					"install":   {},
					"uninstall": {},
				}),
				UpdatedAt: time.Now(),
			},
			"composer": {
				Tool: "composer",
				Tree: treeBranch(map[string]*TreeNode{
					"config": treeBranch(map[string]*TreeNode{
						"allow-plugins": {},
						"cache-dir":     {},
						"repositories":  {},
					}),
					"install": {},
					"require": {},
				}),
				UpdatedAt: time.Now(),
			},
			"brew": {
				Tool: "brew",
				Tree: treeBranch(map[string]*TreeNode{
					"services": treeBranch(map[string]*TreeNode{
						"list":    {},
						"restart": {},
						"start":   {},
						"stop":    {},
					}),
				}),
				UpdatedAt: time.Now(),
			},
			"terraform": {
				Tool: "terraform",
				Tree: treeBranch(map[string]*TreeNode{
					"state": treeBranch(map[string]*TreeNode{
						"list": {},
						"mv":   {},
						"rm":   {},
						"show": {},
					}),
				}),
				UpdatedAt: time.Now(),
			},
			"helm": {
				Tool: "helm",
				Tree: treeBranch(map[string]*TreeNode{
					"repo": treeBranch(map[string]*TreeNode{
						"add":    {},
						"list":   {},
						"remove": {},
						"update": {},
					}),
				}),
				UpdatedAt: time.Now(),
			},
		},
		cacheExpiry: 7 * 24 * time.Hour,
	}

	tests := []struct {
		name   string
		tool   string
		prefix []string
		want   []string
	}{
		{name: "pip cache", tool: "pip", prefix: []string{"cache"}, want: []string{"dir", "info", "list", "purge", "remove"}},
		{name: "composer config", tool: "composer", prefix: []string{"config"}, want: []string{"allow-plugins", "cache-dir", "repositories"}},
		{name: "brew services", tool: "brew", prefix: []string{"services"}, want: []string{"list", "restart", "start", "stop"}},
		{name: "terraform state", tool: "terraform", prefix: []string{"state"}, want: []string{"list", "mv", "rm", "show"}},
		{name: "helm repo", tool: "helm", prefix: []string{"repo"}, want: []string{"add", "list", "remove", "update"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.GetChildren(tt.tool, tt.prefix)
			for _, want := range tt.want {
				if !hasString(got, want) {
					t.Fatalf("GetChildren(%q, %v) missing %q in %v", tt.tool, tt.prefix, want, got)
				}
			}
		})
	}
}

func TestToolTreeRegistry_CachedThreeLevelCloudCoverage(t *testing.T) {
	r := &ToolTreeRegistry{
		trees: map[string]*ToolTreeCache{
			"aws": {
				Tool: "aws",
				Tree: treeBranch(map[string]*TreeNode{
					"cloudformation": treeBranch(map[string]*TreeNode{
						"wait": treeBranch(map[string]*TreeNode{
							"stack-create-complete": {},
							"stack-update-complete": {},
						}),
					}),
				}),
				UpdatedAt: time.Now(),
			},
			"gcloud": {
				Tool: "gcloud",
				Tree: treeBranch(map[string]*TreeNode{
					"container": treeBranch(map[string]*TreeNode{
						"clusters": treeBranch(map[string]*TreeNode{
							"get-credentials": {},
							"list":            {},
						}),
					}),
				}),
				UpdatedAt: time.Now(),
			},
			"az": {
				Tool: "az",
				Tree: treeBranch(map[string]*TreeNode{
					"storage": treeBranch(map[string]*TreeNode{
						"account": treeBranch(map[string]*TreeNode{
							"list": {},
							"show": {},
						}),
					}),
					"network": treeBranch(map[string]*TreeNode{
						"vnet": treeBranch(map[string]*TreeNode{
							"list": {},
							"subnet": treeBranch(map[string]*TreeNode{
								"create": {},
								"list":   {},
							}),
						}),
					}),
				}),
				UpdatedAt: time.Now(),
			},
		},
		cacheExpiry: 7 * 24 * time.Hour,
	}

	tests := []struct {
		name   string
		tool   string
		prefix []string
		want   []string
	}{
		{name: "aws cloudformation wait", tool: "aws", prefix: []string{"cloudformation", "wait"}, want: []string{"stack-create-complete", "stack-update-complete"}},
		{name: "gcloud container clusters", tool: "gcloud", prefix: []string{"container", "clusters"}, want: []string{"get-credentials", "list"}},
		{name: "az storage account", tool: "az", prefix: []string{"storage", "account"}, want: []string{"list", "show"}},
		{name: "az network vnet", tool: "az", prefix: []string{"network", "vnet"}, want: []string{"list", "subnet"}},
		{name: "az network vnet subnet", tool: "az", prefix: []string{"network", "vnet", "subnet"}, want: []string{"create", "list"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.GetChildren(tt.tool, tt.prefix)
			for _, want := range tt.want {
				if !hasString(got, want) {
					t.Fatalf("GetChildren(%q, %v) missing %q in %v", tt.tool, tt.prefix, want, got)
				}
			}
		})
	}
}

func TestGet_ExpiredCache(t *testing.T) {
	tmpDir := t.TempDir()
	r := &ToolTreeRegistry{
		trees: map[string]*ToolTreeCache{
			"git": {
				Tool: "git",
				Tree: treeBranch(map[string]*TreeNode{
					"add":    {},
					"commit": {},
				}),
				UpdatedAt: time.Now().Add(-8 * 24 * time.Hour),
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
	r := NewToolTreeRegistry(tmpDir)

	subcommands := r.Get("nonexistenttool12345")
	if subcommands != nil {
		t.Errorf("Expected nil for nonexistent tool, got %v", subcommands)
	}
}

func TestFetchSubcommands_NonexistentTool(t *testing.T) {
	r := &ToolTreeRegistry{
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

func TestParseAWSHelp(t *testing.T) {
	output := `AWS CLI

SERVICES
  s3          Amazon Simple Storage Service
  ec2         Amazon Elastic Compute Cloud

AVAILABLE COMMANDS
  configure   Configure the AWS CLI
`

	subcommands := parseAWSHelp(output)
	expected := []string{"s3", "ec2", "configure"}
	if len(subcommands) != len(expected) {
		t.Fatalf("Expected %d aws subcommands, got %d: %v", len(expected), len(subcommands), subcommands)
	}
	for i, cmd := range expected {
		if subcommands[i] != cmd {
			t.Fatalf("Expected aws subcommands[%d] = %s, got %s", i, cmd, subcommands[i])
		}
	}
}

func TestParseGCloudHelp(t *testing.T) {
	output := `NAME
    gcloud - manage Google Cloud resources

GROUPS
    compute     Read and write Compute Engine resources
    auth        Manage authentication

COMMANDS
    config      View and edit configuration
`

	subcommands := parseGCloudHelp(output)
	expected := []string{"compute", "auth", "config"}
	if len(subcommands) != len(expected) {
		t.Fatalf("Expected %d gcloud subcommands, got %d: %v", len(expected), len(subcommands), subcommands)
	}
	for i, cmd := range expected {
		if subcommands[i] != cmd {
			t.Fatalf("Expected gcloud subcommands[%d] = %s, got %s", i, cmd, subcommands[i])
		}
	}
}

func TestParseAzureHelp(t *testing.T) {
	output := `Group
    az

Subgroups:
  group        Manage resource groups.

Commands:
  login        Log in to Azure.
  account      Manage Azure subscription information.
`

	subcommands := parseAzureHelp(output)
	expected := []string{"group", "login", "account"}
	if len(subcommands) != len(expected) {
		t.Fatalf("Expected %d az subcommands, got %d: %v", len(expected), len(subcommands), subcommands)
	}
	for i, cmd := range expected {
		if subcommands[i] != cmd {
			t.Fatalf("Expected az subcommands[%d] = %s, got %s", i, cmd, subcommands[i])
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
	r := &ToolTreeRegistry{}

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
		{"aws", true},
		{"gcloud", true},
		{"az", true},
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
	r := NewToolTreeRegistry(tmpDir)

	r.PreFetch()
}

func TestFetchSubcommands_ToolNotInPath(t *testing.T) {
	r := &ToolTreeRegistry{
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
	}

	result := r.fetchSubcommands("nonexistentcommand12345xyz")
	if result != nil {
		t.Errorf("Expected nil for tool not in PATH, got %v", result)
	}
}

func TestFetchSubcommands_DefaultParser(t *testing.T) {
	r := &ToolTreeRegistry{
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
	}

	result := r.fetchSubcommands("ls")
	_ = result
}

func TestGet_WithFetch(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewToolTreeRegistry(tmpDir)

	_ = r.Get("ls")
}

func TestGet_CacheExpiredAndRefetch(t *testing.T) {
	tmpDir := t.TempDir()
	r := &ToolTreeRegistry{
		trees: map[string]*ToolTreeCache{
			"ls": {
				Tool:      "ls",
				Tree:      treeBranch(map[string]*TreeNode{"old": {}}),
				UpdatedAt: time.Now().Add(-10 * 24 * time.Hour),
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

	r := &ToolTreeRegistry{
		trees: map[string]*ToolTreeCache{
			"git": {
				Tool:      "git",
				Tree:      treeBranch(map[string]*TreeNode{"add": {}}),
				UpdatedAt: time.Now(),
			},
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
	r := &ToolTreeRegistry{
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
	r := &ToolTreeRegistry{
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
	}
	result := r.fetchSubcommands("docker")
	_ = result
}

func TestFetchSubcommands_Npm(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	defer os.Setenv("PATH", oldPath)

	npmPath := filepath.Join(tmpDir, "npm")
	script := "#!/bin/sh\nif [ \"$1\" = \"--help\" ]; then\n  printf '%s\\n' \\\n    'npm <command>' \\\n    '' \\\n    'All commands:' \\\n    '' \\\n    '    access, adduser, audit, bugs, cache, ci, completion,' \\\n    '    config, dedupe, deprecate, diff, dist-tag, docs, doctor,' \\\n    '    edit, exec, explain, explore, find-dupes, fund, get, help,' \\\n    '    help-search, hook, init, install, install-ci-test,' \\\n    '    install-test, link, ll, login, logout, ls, org, outdated,' \\\n    '    owner, pack, ping, pkg, prefix, profile, prune, publish,' \\\n    '    query, rebuild, repo, restart, root, run-script, sbom,' \\\n    '    search, set, shrinkwrap, star, stars, start, stop, team,' \\\n    '    test, token, uninstall, unpublish, unstar, update, version,' \\\n    '    view, whoami'\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(npmPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write npm stub: %v", err)
	}

	if err := os.Setenv("PATH", tmpDir); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}

	r := &ToolTreeRegistry{
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
		helpTimeout: 10 * time.Second,
	}
	result := r.fetchSubcommands("npm")
	if len(result) == 0 {
		t.Fatalf("Expected npm subcommands")
	}

	expectedCmds := []string{"install", "config", "test", "publish"}
	actualCmds := make(map[string]bool)
	for _, cmd := range result {
		actualCmds[cmd] = true
	}
	for _, expected := range expectedCmds {
		if !actualCmds[expected] {
			t.Errorf("Expected npm to have subcommand '%s', but it was missing", expected)
		}
	}
}

func TestFetchSubcommands_Go(t *testing.T) {
	if GetPath("go") == "" {
		t.Skip("go not found in PATH")
	}
	r := &ToolTreeRegistry{
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
	r := &ToolTreeRegistry{
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
	r := &ToolTreeRegistry{
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
	r := &ToolTreeRegistry{
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
	}
	result := r.fetchSubcommands("yarn")
	_ = result
}

func TestGetHelpOutput_GitPrefersHelpA(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()

	gitPath := filepath.Join(tmpDir, "git")
	script := "#!/bin/sh\nif [ \"$1\" = \"help\" ] && [ \"$2\" = \"-a\" ]; then\n  echo git-help-a\n  exit 0\nfi\nif [ \"$1\" = \"--help\" ]; then\n  echo git-help\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(gitPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write git stub: %v", err)
	}

	if err := os.Setenv("PATH", tmpDir); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}
	r := &ToolTreeRegistry{helpTimeout: 10 * time.Second}
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
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()

	toolPath := filepath.Join(tmpDir, "mytool")
	script := "#!/bin/sh\nif [ \"$1\" = \"--help\" ]; then\n  exit 1\nfi\nif [ \"$1\" = \"help\" ]; then\n  echo fallback-help\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(toolPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write tool stub: %v", err)
	}

	if err := os.Setenv("PATH", tmpDir); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}
	r := &ToolTreeRegistry{helpTimeout: 10 * time.Second}
	output, err := r.getHelpOutput("mytool")
	if err != nil {
		t.Fatalf("getHelpOutput failed: %v", err)
	}
	if strings.TrimSpace(output) != "fallback-help" {
		t.Fatalf("Expected help fallback output, got %q", output)
	}
}

func TestGetHelpOutputAtPath_CloudCommands(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()

	scripts := map[string]string{
		"aws":    "#!/bin/sh\nif [ \"$1\" = \"s3\" ] && [ \"$2\" = \"help\" ]; then\n  echo aws-s3-help\n  exit 0\nfi\nexit 1\n",
		"gcloud": "#!/bin/sh\nif [ \"$1\" = \"compute\" ] && [ \"$2\" = \"--help\" ]; then\n  echo gcloud-compute-help\n  exit 0\nfi\nexit 1\n",
		"az":     "#!/bin/sh\nif [ \"$1\" = \"group\" ] && [ \"$2\" = \"--help\" ]; then\n  echo az-group-help\n  exit 0\nfi\nexit 1\n",
	}
	for name, script := range scripts {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(script), 0755); err != nil {
			t.Fatalf("Failed to write %s stub: %v", name, err)
		}
	}

	if err := os.Setenv("PATH", tmpDir); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}
	r := &ToolTreeRegistry{helpTimeout: 10 * time.Second}

	tests := []struct {
		tool   string
		prefix []string
		want   string
	}{
		{tool: "aws", prefix: []string{"s3"}, want: "aws-s3-help"},
		{tool: "gcloud", prefix: []string{"compute"}, want: "gcloud-compute-help"},
		{tool: "az", prefix: []string{"group"}, want: "az-group-help"},
	}

	for _, tt := range tests {
		output, err := r.getHelpOutputAtPath(tt.tool, tt.prefix...)
		if err != nil {
			t.Fatalf("getHelpOutputAtPath(%s, %v) failed: %v", tt.tool, tt.prefix, err)
		}
		if strings.TrimSpace(output) != tt.want {
			t.Fatalf("Expected %s help output %q, got %q", tt.tool, tt.want, output)
		}
	}
}

func TestGetHelpOutput_BrewUsesCommands(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()

	brewPath := filepath.Join(tmpDir, "brew")
	script := "#!/bin/sh\nif [ \"$1\" = \"commands\" ]; then\n  echo install\n  echo upgrade\n  exit 0\nfi\nif [ \"$1\" = \"--help\" ]; then\n  echo should-not-be-used\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(brewPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write brew stub: %v", err)
	}

	if err := os.Setenv("PATH", tmpDir); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}
	r := &ToolTreeRegistry{helpTimeout: 10 * time.Second}
	output, err := r.getHelpOutput("brew")
	if err != nil {
		t.Fatalf("getHelpOutput failed: %v", err)
	}
	if strings.Contains(output, "should-not-be-used") || !strings.Contains(output, "install") {
		t.Fatalf("Expected brew commands output, got %q", output)
	}
}

func TestGetHelpOutput_GitFallsBackToHelp(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()

	gitPath := filepath.Join(tmpDir, "git")
	script := "#!/bin/sh\nif [ \"$1\" = \"help\" ] && [ \"$2\" = \"-a\" ]; then\n  exit 1\nfi\nif [ \"$1\" = \"--help\" ]; then\n  echo git-help\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(gitPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write git fallback stub: %v", err)
	}

	if err := os.Setenv("PATH", tmpDir); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}
	r := &ToolTreeRegistry{helpTimeout: 10 * time.Second}
	output, err := r.getHelpOutput("git")
	if err != nil {
		t.Fatalf("getHelpOutput fallback failed: %v", err)
	}
	if strings.TrimSpace(output) != "git-help" {
		t.Fatalf("Expected git --help fallback output, got %q", output)
	}
}

func TestGetHelpOutput_BrewFallsBackToHelp(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()

	brewPath := filepath.Join(tmpDir, "brew")
	script := "#!/bin/sh\nif [ \"$1\" = \"commands\" ]; then\n  exit 1\nfi\nif [ \"$1\" = \"--help\" ]; then\n  echo fallback-brew-help\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(brewPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write brew fallback stub: %v", err)
	}

	if err := os.Setenv("PATH", tmpDir); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}
	r := &ToolTreeRegistry{helpTimeout: 10 * time.Second}
	output, err := r.getHelpOutput("brew")
	if err != nil {
		t.Fatalf("getHelpOutput brew fallback failed: %v", err)
	}
	if strings.TrimSpace(output) != "fallback-brew-help" {
		t.Fatalf("Expected brew --help fallback output, got %q", output)
	}
}

func TestGetHelpOutput_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()

	toolPath := filepath.Join(tmpDir, "slowtool")
	script := "#!/bin/sh\nif [ \"$1\" = \"--help\" ]; then\n  /bin/sleep 1\n  echo slow-help\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(toolPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write slow tool stub: %v", err)
	}

	if err := os.Setenv("PATH", tmpDir); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}
	r := &ToolTreeRegistry{helpTimeout: 50 * time.Millisecond}

	start := time.Now()
	output, err := r.getHelpOutput("slowtool")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timed out help command to return an error")
	}
	if output != "" {
		t.Fatalf("Expected empty output after timeout, got %q", output)
	}
	if elapsed >= 500*time.Millisecond {
		t.Fatalf("Expected timeout to return quickly, got %v", elapsed)
	}
}

func TestGetHelpOutput_GitTimeoutStopsFallback(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()

	gitPath := filepath.Join(tmpDir, "git")
	script := "#!/bin/sh\nif [ \"$1\" = \"help\" ] && [ \"$2\" = \"-a\" ]; then\n  /bin/sleep 1\n  exit 0\nfi\nif [ \"$1\" = \"--help\" ]; then\n  echo should-not-run\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(gitPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write git timeout stub: %v", err)
	}

	if err := os.Setenv("PATH", tmpDir); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}
	r := &ToolTreeRegistry{helpTimeout: 50 * time.Millisecond}
	output, err := r.getHelpOutput("git")
	if err == nil {
		t.Fatal("Expected git help timeout to return an error")
	}
	if output != "" {
		t.Fatalf("Expected timed out git help to return empty output, got %q", output)
	}
}

func TestGetHelpOutput_BrewTimeoutStopsFallback(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()

	brewPath := filepath.Join(tmpDir, "brew")
	script := "#!/bin/sh\nif [ \"$1\" = \"commands\" ]; then\n  /bin/sleep 1\n  exit 0\nfi\nif [ \"$1\" = \"--help\" ]; then\n  echo should-not-run\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(brewPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write brew timeout stub: %v", err)
	}

	if err := os.Setenv("PATH", tmpDir); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}
	r := &ToolTreeRegistry{helpTimeout: 50 * time.Millisecond}
	output, err := r.getHelpOutput("brew")
	if err == nil {
		t.Fatal("Expected brew commands timeout to return an error")
	}
	if output != "" {
		t.Fatalf("Expected timed out brew commands to return empty output, got %q", output)
	}
}

func TestFetchSubcommands_DefaultParserWithStub(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()

	toolPath := filepath.Join(tmpDir, "mytool")
	script := "#!/bin/sh\nif [ \"$1\" = \"--help\" ]; then\n  printf 'Usage: mytool <command>\\n\\n  start    Start service\\n  stop     Stop service\\n'\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(toolPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write tool stub: %v", err)
	}

	if err := os.Setenv("PATH", tmpDir); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}
	r := &ToolTreeRegistry{
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
		helpTimeout: 10 * time.Second,
	}

	subcommands := r.fetchSubcommands("mytool")
	if len(subcommands) != 2 || subcommands[0] != "start" || subcommands[1] != "stop" {
		t.Fatalf("Expected generic parser subcommands, got %v", subcommands)
	}
}

func TestFetchSubcommands_GCloudNestedWithStub(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()

	toolPath := filepath.Join(tmpDir, "gcloud")
	script := `#!/bin/sh
if [ "$1" = "--help" ]; then
  printf 'GROUPS\n    compute     Read and write Compute Engine resources\n'
  exit 0
fi
if [ "$1" = "compute" ] && [ "$2" = "--help" ]; then
  printf 'GROUPS\n    instances   Read and write Compute Engine VM instances\n'
  exit 0
fi
if [ "$1" = "compute" ] && [ "$2" = "instances" ] && [ "$3" = "--help" ]; then
  printf 'COMMANDS\n    list        List Compute Engine VM instances\n'
  exit 0
fi
exit 1
`
	if err := os.WriteFile(toolPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write gcloud stub: %v", err)
	}

	if err := os.Setenv("PATH", tmpDir); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}
	r := &ToolTreeRegistry{
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
		helpTimeout: 10 * time.Second,
	}

	root := r.fetchSubcommands("gcloud")
	if len(root) != 1 || root[0] != "compute" {
		t.Fatalf("Expected gcloud root subcommands, got %v", root)
	}

	children := r.fetchSubcommands("gcloud", "compute")
	if len(children) != 1 || children[0] != "instances" {
		t.Fatalf("Expected gcloud compute children, got %v", children)
	}

	grandChildren := r.fetchSubcommands("gcloud", "compute", "instances")
	if len(grandChildren) != 1 || grandChildren[0] != "list" {
		t.Fatalf("Expected gcloud compute instances children, got %v", grandChildren)
	}
}

func TestFetchSubcommands_EmptyHelpOutput(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()

	toolPath := filepath.Join(tmpDir, "emptytool")
	script := "#!/bin/sh\nif [ \"$1\" = \"--help\" ]; then\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(toolPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write empty tool stub: %v", err)
	}

	if err := os.Setenv("PATH", tmpDir); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}
	r := &ToolTreeRegistry{
		cacheDir:    "",
		cacheExpiry: 7 * 24 * time.Hour,
		helpTimeout: 10 * time.Second,
	}

	if got := r.fetchSubcommands("emptytool"); got != nil {
		t.Fatalf("Expected empty help output to return nil subcommands, got %v", got)
	}
}
