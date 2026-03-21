package commands

import (
	"os"
	"path/filepath"
	"strings"
)

// Discover discovers commands from the system PATH.
func Discover() []string {
	commands := make(map[string]bool)

	path := os.Getenv("PATH")
	if path == "" {
		return []string{}
	}

	paths := strings.Split(path, string(os.PathListSeparator))
	for _, p := range paths {
		discoverInDir(p, commands)
	}

	// Convert to slice
	result := make([]string, 0, len(commands))
	for cmd := range commands {
		result = append(result, cmd)
	}

	return result
}

func discoverInDir(dir string, commands map[string]bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Check if executable
		mode := info.Mode()
		if mode&0111 == 0 {
			continue
		}

		name := entry.Name()
		// Skip hidden files
		if strings.HasPrefix(name, ".") {
			continue
		}

		// Remove common extensions on Windows
		name = strings.TrimSuffix(name, ".exe")
		name = strings.TrimSuffix(name, ".bat")
		name = strings.TrimSuffix(name, ".cmd")

		commands[name] = true
	}
}

// DiscoverCommon returns a list of common commands.
// This is used as a fallback when PATH discovery fails.
func DiscoverCommon() []string {
	return []string{
		"git", "docker", "npm", "yarn", "node", "python", "python3", "pip", "pip3",
		"go", "cargo", "rustc", "make", "cmake", "gcc", "clang",
		"ls", "cd", "cat", "grep", "find", "sed", "awk", "tail", "head",
		"curl", "wget", "ssh", "scp", "rsync",
		"tar", "zip", "unzip", "gzip",
		"ps", "kill", "top", "htop",
		"sudo", "su", "chmod", "chown",
		"mkdir", "rm", "cp", "mv", "touch",
		"echo", "printf", "env", "export",
		"kubectl", "helm", "terraform", "ansible",
	}
}

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
		fullPath := filepath.Join(p, name)
		if IsExecutable(fullPath) {
			return fullPath
		}
	}

	return ""
}
