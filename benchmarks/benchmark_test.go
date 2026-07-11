package benchmarks

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type cliBenchmarkEnv struct {
	root      string
	home      string
	tmpDir    string
	binDir    string
	binary    string
	gitStderr string
}

func BenchmarkTypoCLI(b *testing.B) {
	env := newCLIBenchmarkEnv(b)
	b.ReportAllocs()

	benchmarks := []struct {
		name         string
		args         []string
		expectedCode int
		expectedOut  string
	}{
		{
			name:         "fix-rule-match",
			args:         []string{"fix", "--no-history", "gut status"},
			expectedCode: 0,
			expectedOut:  "git status\n",
		},
		{
			name:         "fix-distance-match",
			args:         []string{"fix", "--no-history", "dkcer ps"},
			expectedCode: 0,
			expectedOut:  "docker ps\n",
		},
		{
			name:         "fix-compound-multi-match",
			args:         []string{"fix", "--no-history", "gti stauts && dcoker ps"},
			expectedCode: 0,
			expectedOut:  "git status && docker ps\n",
		},
		{
			name:         "fix-pipeline-compound-match",
			args:         []string{"fix", "--no-history", "gut status | grep main && dcoker ps"},
			expectedCode: 0,
			expectedOut:  "git status | grep main && docker ps\n",
		},
		{
			name:         "fix-wrapper-subcommand-match",
			args:         []string{"fix", "--no-history", "sudo git -C repo stauts || dcoker ps"},
			expectedCode: 0,
			expectedOut:  "sudo git -C repo status || docker ps\n",
		},
		{
			name:         "fix-parser-compound-match",
			args:         []string{"fix", "--no-history", "-s", env.gitStderr, "sudo git remove -v && dcoker ps"},
			expectedCode: 0,
			expectedOut:  "sudo git remote -v && docker ps\n",
		},
		{
			name:         "fix-no-match",
			args:         []string{"fix", "--no-history", "xyzabc"},
			expectedCode: 1,
		},
		{
			name:         "fix-noop-compound",
			args:         []string{"fix", "--no-history", "git status && echo ok"},
			expectedCode: 1,
		},
		{
			name:         "rules-list",
			args:         []string{"rules", "list"},
			expectedCode: 0,
		},
		{
			name:         "version",
			args:         []string{"version"},
			expectedCode: 0,
		},
	}

	for _, tt := range benchmarks {
		b.Run(tt.name, func(b *testing.B) {
			env.validate(b, tt.expectedCode, tt.expectedOut, tt.args...)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				env.run(b, tt.expectedCode, tt.args...)
			}
		})
	}
}

func (e *cliBenchmarkEnv) validate(b *testing.B, expectedCode int, expectedOut string, args ...string) {
	b.Helper()

	cmd := exec.Command(e.binary, args...)
	cmd.Dir = e.root
	cmd.Env = e.commandEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	code, err := runBenchmarkCommand(cmd)
	if err != nil {
		b.Fatalf("failed to validate benchmark command: %v", err)
	}
	if code != expectedCode || (expectedOut != "" && stdout.String() != expectedOut) {
		b.Fatalf(
			"invalid benchmark case %q: code=%d want=%d stdout=%q want=%q stderr=%q",
			strings.Join(args, " "),
			code,
			expectedCode,
			stdout.String(),
			expectedOut,
			stderr.String(),
		)
	}
}

func newCLIBenchmarkEnv(b *testing.B) *cliBenchmarkEnv {
	b.Helper()

	root := benchmarkRepoRoot(b)
	base := b.TempDir()
	home := filepath.Join(base, "home")
	tmpDir := filepath.Join(base, "tmp")
	binDir := filepath.Join(base, "bin")

	for _, dir := range []string{home, tmpDir, binDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			b.Fatalf("failed to create benchmark directory: %v", err)
		}
	}

	binary := filepath.Join(binDir, "typo")
	build := exec.Command("go", "build", "-o", binary, "./cmd/typo")
	build.Dir = root
	output, err := build.CombinedOutput()
	if err != nil {
		b.Fatalf("failed to build benchmark binary: %v\n%s", err, output)
	}

	env := &cliBenchmarkEnv{
		root:   root,
		home:   home,
		tmpDir: tmpDir,
		binDir: binDir,
		binary: binary,
	}

	env.seedCommandStubs(
		b,
		"git", "docker", "npm", "kubectl", "brew", "yarn", "cargo", "helm",
		"terraform", "sudo", "grep", "echo", "true", "env",
	)
	env.seedSubcommands(b, map[string][]string{
		"git":     {"status", "remote", "commit", "checkout", "branch", "pull", "push"},
		"docker":  {"build", "ps", "images", "run", "logs", "compose"},
		"npm":     {"install", "list", "run", "test", "ci"},
		"kubectl": {"apply", "describe", "get", "logs"},
		"brew":    {"install", "update", "upgrade", "list"},
	})
	env.gitStderr = env.writeTempFile(
		b,
		"git.stderr",
		"git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n",
	)

	return env
}

func benchmarkRepoRoot(b *testing.B) string {
	b.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatal("failed to locate benchmark source file")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(filename), ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		b.Fatalf("failed to locate repository root: %v", err)
	}

	return root
}

func (e *cliBenchmarkEnv) configDir() string {
	return filepath.Join(e.home, ".typo")
}

func (e *cliBenchmarkEnv) seedCommandStubs(b *testing.B, names ...string) {
	b.Helper()

	for _, name := range names {
		path := filepath.Join(e.binDir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
			b.Fatalf("failed to write command stub %s: %v", name, err)
		}
	}
}

func (e *cliBenchmarkEnv) seedSubcommands(b *testing.B, tools map[string][]string) {
	b.Helper()

	if err := os.MkdirAll(e.configDir(), 0755); err != nil {
		b.Fatalf("failed to create config directory: %v", err)
	}

	data, err := marshalBenchmarkToolTreeCache(tools, time.Now())
	if err != nil {
		b.Fatalf("failed to marshal subcommand cache: %v", err)
	}

	cacheFile := filepath.Join(e.configDir(), "subcommands.json")
	if err := os.WriteFile(cacheFile, data, 0600); err != nil {
		b.Fatalf("failed to write subcommand cache: %v", err)
	}
}

func (e *cliBenchmarkEnv) writeTempFile(b *testing.B, name, content string) string {
	b.Helper()

	path := filepath.Join(e.tmpDir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		b.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

func (e *cliBenchmarkEnv) commandEnv() []string {
	filtered := make([]string, 0, len(os.Environ())+4)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, "HOME=") ||
			strings.HasPrefix(item, "PATH=") ||
			strings.HasPrefix(item, "TMPDIR=") ||
			strings.HasPrefix(item, "ZDOTDIR=") {
			continue
		}
		filtered = append(filtered, item)
	}

	filtered = append(filtered,
		"HOME="+e.home,
		"PATH="+e.binDir,
		"TMPDIR="+e.tmpDir,
		"ZDOTDIR="+e.home,
	)

	return filtered
}

func (e *cliBenchmarkEnv) run(b *testing.B, expectedCode int, args ...string) {
	b.Helper()

	cmd := exec.Command(e.binary, args...)
	cmd.Dir = e.root
	cmd.Env = e.commandEnv()
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	code, err := runBenchmarkCommand(cmd)
	if err != nil {
		b.Fatalf("failed to execute benchmark command: %v", err)
	}

	if code == expectedCode {
		return
	}

	debug := exec.Command(e.binary, args...)
	debug.Dir = e.root
	debug.Env = e.commandEnv()

	var stdout, stderr bytes.Buffer
	debug.Stdout = &stdout
	debug.Stderr = &stderr
	_ = debug.Run()

	b.Fatalf(
		"unexpected exit code for %q: got=%d want=%d stdout=%q stderr=%q",
		strings.Join(args, " "),
		code,
		expectedCode,
		stdout.String(),
		stderr.String(),
	)
}

func runBenchmarkCommand(cmd *exec.Cmd) (int, error) {
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 0, err
}
