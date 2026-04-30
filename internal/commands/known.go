package commands

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/yuluo-yx/typo/internal/utils"
)

const maxPathDiscoveryWorkers = 16

// Discover discovers commands from the system PATH.
func Discover() []string {
	commands := make(map[string]bool)

	path := os.Getenv("PATH")
	if path == "" {
		return []string{}
	}

	paths := strings.Split(path, string(os.PathListSeparator))
	workerCount := pathDiscoveryWorkerCount(len(paths))
	if workerCount == 0 {
		return []string{}
	}

	jobs := make(chan string)
	results := make(chan []string, workerCount)

	var wg sync.WaitGroup
	wg.Add(workerCount)
	for range workerCount {
		go func() {
			defer wg.Done()
			for p := range jobs {
				if names := discoverNamesInDir(p); len(names) > 0 {
					results <- names
				}
			}
		}()
	}

	go func() {
		for _, p := range paths {
			jobs <- p
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	for names := range results {
		for _, cmd := range names {
			commands[cmd] = true
		}
	}

	// Convert to slice
	result := make([]string, 0, len(commands))
	for cmd := range commands {
		result = append(result, cmd)
	}

	sort.Strings(result)

	return result
}

func discoverInDir(dir string, commands map[string]bool) {
	for _, name := range discoverNamesInDir(dir) {
		commands[name] = true
	}
}

func pathDiscoveryWorkerCount(pathCount int) int {
	if pathCount <= 0 {
		return 0
	}

	workerCount := runtime.NumCPU() * 2
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > maxPathDiscoveryWorkers {
		workerCount = maxPathDiscoveryWorkers
	}
	if workerCount > pathCount {
		workerCount = pathCount
	}
	return workerCount
}

func discoverNamesInDir(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		name := entry.Name()
		// Skip hidden files
		if strings.HasPrefix(name, ".") {
			continue
		}

		if runtime.GOOS == "windows" {
			if !isWindowsExecutablePath(name) {
				continue
			}
		} else {
			mode := info.Mode()
			if mode&0111 == 0 {
				continue
			}
		}

		name = trimExecutableSuffix(name)

		names = append(names, name)
	}
	return names
}

// DiscoverCommon returns a list of common commands.
// This is used as a fallback when PATH discovery fails.
func DiscoverCommon() []string {
	return []string{
		"git", "docker", "npm", "yarn", "node", "python", "python3", "pip", "pip3",
		"go", "cargo", "rustc", "make", "cmake", "gcc", "clang",
		"ls", "cd", "pwd", "cat", "grep", "find", "sed", "awk", "tail", "head", "xargs",
		"sort", "uniq", "cut", "tee", "wc", "which", "less",
		"curl", "wget", "ssh", "scp", "rsync",
		"tar", "zip", "unzip", "gzip", "ln", "du", "df", "date", "open",
		"clear", "man", "whoami", "uname", "basename", "dirname", "file", "stat",
		"ps", "kill", "top", "htop",
		"sudo", "su", "chmod", "chown",
		"mkdir", "rm", "cp", "mv", "touch",
		"echo", "printf", "env", "export",
		"kubectl", "helm", "terraform", "ansible",
		"terragrunt", "terramate", "opentofu", "tofu", "pulumi", "cdktf",
		"crossplane", "packer", "vault", "consul", "nomad",
		"aws", "sam", "cdk", "eksctl",
		"gcloud", "gsutil",
		"az", "func", "azd",
		"doctl", "oci", "linode-cli",
	}
}

var commonCommandSet = utils.StringSet(DiscoverCommon())

// ShellBuiltins returns a list of shell builtin commands.
// These commands are built into the shell and not found in PATH.
func ShellBuiltins() []string {
	return []string{
		"source", ".", "alias", "unalias", "exit", "return", "test",
		"cd", "pushd", "popd", "dirs", "echo", "printf",
		"export", "unset", "readonly", "read",
		"set", "shift", "break", "continue",
		"true", "false", ":", "[", "[[",
		"eval", "exec", "trap", "wait",
		"jobs", "fg", "bg", "disown",
		"history", "logout", "login",
		"type", "hash", "help", "local",
		"times", "ulimit", "umask",
	}
}

var shellBuiltinSet = utils.StringSet(ShellBuiltins())

// Filter filters commands by prefix.
func Filter(commands []string, prefix string) []string {
	if prefix == "" {
		return commands
	}

	result := make([]string, 0)
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, prefix) {
			result = append(result, cmd)
		}
	}
	return result
}

// AddFileExtension adds the appropriate file extension for the current OS.
func AddFileExtension(name string) string {
	return name // Let the shell handle extensions
}

// IsExecutable checks if a file is executable.
func IsExecutable(path string) bool {
	cleanPath := filepath.Clean(path)
	// #nosec G703 -- path is normalized and only checked for executable bit.
	info, err := os.Stat(cleanPath)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		ext := strings.ToLower(filepath.Ext(cleanPath))
		return ext != "" && isWindowsExecutablePath(cleanPath)
	}
	return info.Mode()&0111 != 0
}

// GetPath returns the full path to a command.
func GetPath(name string) string {
	path := os.Getenv("PATH")
	if path == "" {
		return ""
	}

	paths := strings.Split(path, string(os.PathListSeparator))
	for _, p := range paths {
		for _, candidate := range executableCandidates(name) {
			fullPath := filepath.Join(p, candidate)
			if IsExecutable(fullPath) {
				return fullPath
			}
		}
	}

	return ""
}

// IsCommonCommand reports whether a command is part of the common fallback set.
func IsCommonCommand(name string) bool {
	return commonCommandSet[name]
}

// IsShellBuiltin reports whether a command is a shell builtin.
func IsShellBuiltin(name string) bool {
	return shellBuiltinSet[name]
}

func executableCandidates(name string) []string {
	if runtime.GOOS != "windows" {
		return []string{name}
	}
	if filepath.Ext(name) != "" {
		return []string{name}
	}

	exts := windowsExecutableExtensions()
	candidates := make([]string, 0, len(exts)+1)
	for _, ext := range exts {
		candidates = append(candidates, name+ext)
	}
	return append(candidates, name)
}

func windowsExecutableExtensions() []string {
	value := os.Getenv("PATHEXT")
	if value == "" {
		value = ".com;.exe;.bat;.cmd"
	}

	parts := strings.Split(strings.ToLower(value), ";")
	exts := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		exts = append(exts, part)
	}
	return exts
}

func isWindowsExecutablePath(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return false
	}
	for _, candidate := range windowsExecutableExtensions() {
		if ext == candidate {
			return true
		}
	}
	return false
}

func trimExecutableSuffix(name string) string {
	lowerName := strings.ToLower(name)
	for _, ext := range windowsExecutableExtensions() {
		if strings.HasSuffix(lowerName, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	return name
}
