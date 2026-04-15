package parser

import (
	"regexp"
	"strings"

	"github.com/yuluo-yx/typo/internal/commands"
	itypes "github.com/yuluo-yx/typo/internal/types"
)

// PermissionParser adds a sudo prefix when stderr shows a permission error.
type PermissionParser struct {
	strongPermissionPatterns []*regexp.Regexp
	genericPermissionDenied  *regexp.Regexp
	remoteAuthPatterns       []*regexp.Regexp
	genericPermissionCmds    map[string]bool
	passwordPrompt           *regexp.Regexp
}

// NewPermissionParser creates a parser for permission-related command failures.
func NewPermissionParser() *PermissionParser {
	return &PermissionParser{
		strongPermissionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?im)\boperation not permitted\b`),
			regexp.MustCompile(`(?im)\b(?:EACCES|EPERM)\b`),
			regexp.MustCompile(`(?im)\bmust be (?:superuser|root)\b`),
			regexp.MustCompile(`(?im)\byou (?:must|need to) be root\b`),
			regexp.MustCompile(`(?im)\broot privileges? (?:are )?required\b`),
		},
		genericPermissionDenied: regexp.MustCompile(`(?im)\bpermission denied\b`),
		remoteAuthPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?im)\bpermission denied\s*\(publickey\)\b`),
			regexp.MustCompile(`(?im)\bcould not read from remote repository\b`),
			regexp.MustCompile(`(?im)\bauthentication failed\b`),
			regexp.MustCompile(`(?im)\bauthorization failed\b`),
			regexp.MustCompile(`(?im)\brepository not found\b`),
			regexp.MustCompile(`(?im)\bunauthorized\b`),
			regexp.MustCompile(`(?im)\bforbidden\b`),
		},
		genericPermissionCmds: map[string]bool{
			"mkdir":   true,
			"touch":   true,
			"rm":      true,
			"cp":      true,
			"mv":      true,
			"install": true,
			"ln":      true,
			"chmod":   true,
			"chown":   true,
			"cat":     true,
			"tee":     true,
			"dd":      true,
			"tar":     true,
			"unzip":   true,
			"zip":     true,
			"docker":  true,
		},
		passwordPrompt: regexp.MustCompile(`(?im)^\s*(?:\[sudo\]\s*)?password(?: for [^:]+)?:\s*$`),
	}
}

// Name returns the parser name.
func (p *PermissionParser) Name() string {
	return "permission"
}

// Parse decides whether sudo should be prepended based on stderr output.
func (p *PermissionParser) Parse(ctx itypes.ParserContext) itypes.ParserResult {
	if p.shouldSkipContext(ctx) {
		return itypes.ParserResult{Fixed: false}
	}

	stderr := strings.TrimSpace(ctx.Stderr)
	if p.shouldSkipStderr(stderr) {
		return itypes.ParserResult{Fixed: false}
	}

	cmdWord := firstCommandWord(ctx.Command)
	if cmdWord == "" || commands.IsShellBuiltin(cmdWord) {
		return itypes.ParserResult{Fixed: false}
	}

	if p.matchesAny(stderr, p.remoteAuthPatterns) {
		return itypes.ParserResult{Fixed: false}
	}

	if p.matchesAny(stderr, p.strongPermissionPatterns) {
		return p.sudoResult(ctx.Command)
	}

	if p.genericPermissionDenied.MatchString(stderr) && p.genericPermissionCmds[cmdWord] {
		return p.sudoResult(ctx.Command)
	}

	return itypes.ParserResult{Fixed: false}
}

func (p *PermissionParser) shouldSkipContext(ctx itypes.ParserContext) bool {
	return ctx.ExitCode == 0 ||
		ctx.Stderr == "" ||
		ctx.HasPrivilegeWrapper ||
		ctx.HasMultipleCommands ||
		ctx.HasRedirection ||
		ctx.ShellParseFailed
}

func (p *PermissionParser) shouldSkipStderr(stderr string) bool {
	return stderr == "" || p.passwordPrompt.MatchString(stderr)
}

func (p *PermissionParser) matchesAny(stderr string, patterns []*regexp.Regexp) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(stderr) {
			return true
		}
	}
	return false
}

func (p *PermissionParser) sudoResult(command string) itypes.ParserResult {
	return itypes.ParserResult{
		Fixed:   true,
		Command: "sudo " + command,
		Kind:    itypes.FixKindPermissionSudo,
	}
}

func firstCommandWord(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
