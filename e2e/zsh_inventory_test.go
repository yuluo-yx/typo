package e2e

import (
	"strings"
	"testing"
)

func TestE2EZshInventoryFlows(t *testing.T) {
	env := newE2EEnv(t)
	initScript := env.initZshScript(t)

	tests := []struct {
		name   string
		buffer string
		want   string
	}{
		{
			name:   "builtins and system commands",
			buffer: `echp ok | gerp ok && taill -n 1 app.log`,
			want:   "echo ok | grep ok && tail -n 1 app.log",
		},
		{
			name:   "source and docker commands",
			buffer: `sourc ~/.zshrc && dcoker ps`,
			want:   "source ~/.zshrc && docker ps",
		},
		{
			name:   "mixed supported tools",
			buffer: `kubctl get pods && bre update && helmm list`,
			want:   "kubectl get pods && brew update && helm list",
		},
		{
			name:   "global option subcommands",
			buffer: `terraform -chdir infra valdiate && helm --kube-context prod temlpate chart`,
			want:   "terraform -chdir infra validate && helm --kube-context prod template chart",
		},
		{
			name:   "remote and package commands",
			buffer: `scpp build.tar.gz deploy@example.com:/srv/app/ && chmdo 755 deploy.sh && pip3 instlal typo`,
			want:   "scp build.tar.gz deploy@example.com:/srv/app/ && chmod 755 deploy.sh && pip3 install typo",
		},
		{
			name:   "runtime and process commands",
			buffer: `python33 script.py && nodee server.js && pss aux && killl -9 1234`,
			want:   "python3 script.py && node server.js && ps aux && kill -9 1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := `
zle() { true; }
bindkey() { true; }
source "$1"
BUFFER='` + tt.buffer + `'
_typo_fix_command
[[ "$BUFFER" == "` + tt.want + `" ]] || { print -r -- "$BUFFER"; exit 31; }
print -r -- "$BUFFER"
`

			result := env.runZsh(t, initScript, script)
			if result.code != 0 || !strings.Contains(result.stdout, tt.want) {
				t.Fatalf("zsh inventory fix failed: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
			}
		})
	}
}

func TestE2EBashInventoryFlows(t *testing.T) {
	env := newE2EEnv(t)
	initScript := env.initBashScript(t)

	tests := []struct {
		name   string
		buffer string
		want   string
	}{
		{
			name:   "builtins and system commands",
			buffer: `echp ok | gerp ok && taill -n 1 app.log`,
			want:   "echo ok | grep ok && tail -n 1 app.log",
		},
		{
			name:   "source and docker commands",
			buffer: `sourc ~/.bashrc && dcoker ps`,
			want:   "source ~/.bashrc && docker ps",
		},
		{
			name:   "mixed supported tools",
			buffer: `kubctl get pods && bre update && helmm list`,
			want:   "kubectl get pods && brew update && helm list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := `
source "$1"
trap - DEBUG
READLINE_LINE='` + tt.buffer + `'
_typo_fix_command
[[ "$READLINE_LINE" == "` + tt.want + `" ]] || { printf "%s\n" "$READLINE_LINE"; exit 41; }
printf "%s\n" "$READLINE_LINE"
`

			result := env.runBash(t, initScript, script)
			if result.code != 0 || !strings.Contains(result.stdout, tt.want) {
				t.Fatalf("bash inventory fix failed: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
			}
		})
	}
}

func TestE2EFishInventoryFlows(t *testing.T) {
	env := newE2EEnv(t)
	initScript := env.initFishScript(t)

	tests := []struct {
		name   string
		buffer string
		want   string
	}{
		{
			name:   "builtins and system commands",
			buffer: `echp ok | gerp ok && taill -n 1 app.log`,
			want:   "echo ok | grep ok && tail -n 1 app.log",
		},
		{
			name:   "source and docker commands",
			buffer: `sourc ~/.config/fish/config.fish && dcoker ps`,
			want:   "source ~/.config/fish/config.fish && docker ps",
		},
		{
			name:   "mixed supported tools",
			buffer: `kubctl get pods && bre update && helmm list`,
			want:   "kubectl get pods && brew update && helm list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := `
set -g TYPO_TEST_BUFFER '` + tt.buffer + `'

function commandline
    switch "$argv[1]"
        case -b
            printf "%s\n" "$TYPO_TEST_BUFFER"
        case -r
            set -g TYPO_TEST_BUFFER "$argv[2]"
        case -C
            true
        case -f
            true
    end
end

source "$argv[1]"
_typo_fix_command
test "$TYPO_TEST_BUFFER" = "` + tt.want + `"; or begin; printf "%s\n" "$TYPO_TEST_BUFFER"; exit 51; end
printf "%s\n" "$TYPO_TEST_BUFFER"
`

			result := env.runFish(t, initScript, script)
			if result.code != 0 || !strings.Contains(result.stdout, tt.want) {
				t.Fatalf("fish inventory fix failed: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
			}
		})
	}
}
