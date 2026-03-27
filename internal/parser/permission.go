package parser

import (
	"regexp"
	"strings"

	"github.com/yuluo-yx/typo/internal/commands"
)

// PermissionParser adds a sudo prefix when stderr shows a permission error.
type PermissionParser struct {
	permissionPatterns []*regexp.Regexp
	passwordPrompt     *regexp.Regexp
}

// NewPermissionParser creates a parser for permission-related command failures.
func NewPermissionParser() *PermissionParser {
	return &PermissionParser{
		permissionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?im)\bpermission denied\b`),
			regexp.MustCompile(`(?im)\boperation not permitted\b`),
			regexp.MustCompile(`(?im)\b(?:EACCES|EPERM)\b`),
			regexp.MustCompile(`(?im)\bmust be (?:superuser|root)\b`),
			regexp.MustCompile(`(?im)\byou (?:must|need to) be root\b`),
			regexp.MustCompile(`(?im)\broot privileges? (?:are )?required\b`),
		},
		passwordPrompt: regexp.MustCompile(`(?im)^\s*(?:\[sudo\]\s*)?password(?: for [^:]+)?:\s*$`),
	}
}

// Name returns the parser name.
func (p *PermissionParser) Name() string {
	return "permission"
}

// Parse decides whether sudo should be prepended based on stderr output.
func (p *PermissionParser) Parse(ctx Context) Result {
	if ctx.ExitCode == 0 || ctx.Stderr == "" {
		return Result{Fixed: false}
	}
	if ctx.HasPrivilegeWrapper || ctx.HasMultipleCommands || ctx.HasRedirection || ctx.ShellParseFailed {
		return Result{Fixed: false}
	}

	stderr := strings.TrimSpace(ctx.Stderr)
	if stderr == "" || p.passwordPrompt.MatchString(stderr) {
		return Result{Fixed: false}
	}

	cmdWord := firstCommandWord(ctx.Command)
	if cmdWord == "" || commands.IsShellBuiltin(cmdWord) {
		return Result{Fixed: false}
	}

	for _, pattern := range p.permissionPatterns {
		if pattern.MatchString(stderr) {
			return Result{
				Fixed:   true,
				Command: "sudo " + ctx.Command,
			}
		}
	}

	return Result{Fixed: false}
}

func firstCommandWord(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
