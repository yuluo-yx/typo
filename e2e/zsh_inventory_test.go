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
