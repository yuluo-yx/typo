package e2e

import "testing"

func TestE2EInventory_BuiltinsAndSystemCommands(t *testing.T) {
	env := newE2EEnv(t)

	tests := []struct {
		name    string
		command string
		want    string
	}{
		{name: "source builtin", command: "sourc ~/.zshrc", want: "source ~/.zshrc\n"},
		{name: "echo builtin", command: "echp ok", want: "echo ok\n"},
		{name: "echo builtin transposition", command: "ehco ok", want: "echo ok\n"},
		{name: "printf builtin", command: `printff "%s" ok`, want: "printf \"%s\" ok\n"},
		{name: "export builtin", command: "exportt VAR=1", want: "export VAR=1\n"},
		{name: "unset builtin", command: "unsett VAR", want: "unset VAR\n"},
		{name: "history builtin", command: "historyy", want: "history\n"},
		{name: "type builtin", command: "typ git", want: "type git\n"},
		{name: "hash builtin", command: "hasj git", want: "hash git\n"},
		{name: "help builtin", command: "helpp git", want: "help git\n"},
		{name: "test builtin", command: "testt -f file", want: "test -f file\n"},
		{name: "true builtin", command: "truee", want: "true\n"},
		{name: "false builtin", command: "falsee", want: "false\n"},
		{name: "eval builtin", command: `evall "echo ok"`, want: "eval \"echo ok\"\n"},
		{name: "exec builtin", command: "execc ls", want: "exec ls\n"},
		{name: "ls command", command: "sl -la", want: "ls -la\n"},
		{name: "cd command", command: "dc /tmp", want: "cd /tmp\n"},
		{name: "cat command", command: "cta /tmp/file", want: "cat /tmp/file\n"},
		{name: "grep command", command: "gerp main app.log", want: "grep main app.log\n"},
		{name: "tail command", command: "taill -n 1 app.log", want: "tail -n 1 app.log\n"},
		{name: "head command", command: "headd -n 1 app.log", want: "head -n 1 app.log\n"},
		{name: "mkdir command", command: "mkdi tmpdir", want: "mkdir tmpdir\n"},
		{name: "rm command", command: "rmm file.txt", want: "rm file.txt\n"},
		{name: "cp command", command: "cpx a.txt b.txt", want: "cp a.txt b.txt\n"},
		{name: "mv command", command: "mvv a.txt b.txt", want: "mv a.txt b.txt\n"},
		{name: "touch command", command: "touc file.txt", want: "touch file.txt\n"},
		{name: "find command", command: "fin . -name main.go", want: "find . -name main.go\n"},
		{name: "sed command", command: "sedd -n 1p file.txt", want: "sed -n 1p file.txt\n"},
		{name: "awk command", command: `awkk "{print $1}" file.txt`, want: "awk \"{print $1}\" file.txt\n"},
		{name: "sudo command", command: "sduo ls", want: "sudo ls\n"},
		{name: "make command", command: "maek test", want: "make test\n"},
		{name: "curl command", command: "crul https://example.com", want: "curl https://example.com\n"},
		{name: "tar command", command: "tra -tf archive.tar", want: "tar -tf archive.tar\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := env.run(t, "fix", tt.command)
			if result.code != 0 || result.stdout != tt.want {
				t.Fatalf("unexpected fix result: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
			}
		})
	}
}

func TestE2EInventory_SupportedTools(t *testing.T) {
	env := newE2EEnv(t)

	tests := []struct {
		name    string
		command string
		want    string
	}{
		{name: "git main command", command: "gut status", want: "git status\n"},
		{name: "git subcommand", command: "git stattus", want: "git status\n"},
		{name: "docker main command", command: "dcoker ps", want: "docker ps\n"},
		{name: "docker subcommand", command: "docker biuld", want: "docker build\n"},
		{name: "npm main command", command: "nmp list", want: "npm list\n"},
		{name: "npm subcommand", command: "npm instlal react", want: "npm install react\n"},
		{name: "yarn main command", command: "yran add react", want: "yarn add react\n"},
		{name: "yarn subcommand", command: "yarn instlal react", want: "yarn install react\n"},
		{name: "kubectl main command", command: "kubctl get pods", want: "kubectl get pods\n"},
		{name: "kubectl subcommand", command: "kubectl aplly -f deployment.yaml", want: "kubectl apply -f deployment.yaml\n"},
		{name: "cargo main command", command: "crago build", want: "cargo build\n"},
		{name: "cargo subcommand", command: "cargo chcek", want: "cargo check\n"},
		{name: "go main command", command: "og test ./...", want: "go test ./...\n"},
		{name: "go subcommand", command: "go biuld ./...", want: "go build ./...\n"},
		{name: "pip main command", command: "pi list", want: "pip list\n"},
		{name: "pip subcommand", command: "pip instlal typo", want: "pip install typo\n"},
		{name: "brew main command", command: "bre update", want: "brew update\n"},
		//nolint:misspell // Intentional typo for correction coverage.
		{name: "brew subcommand", command: "brew upgarde", want: "brew upgrade\n"},
		{name: "terraform main command", command: "terrafrom plan", want: "terraform plan\n"},
		{name: "terraform subcommand", command: "terraform valdiate", want: "terraform validate\n"},
		{name: "helm main command", command: "helmm list", want: "helm list\n"},
		{name: "helm subcommand", command: "helm temlpate chart", want: "helm template chart\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := env.run(t, "fix", tt.command)
			if result.code != 0 || result.stdout != tt.want {
				t.Fatalf("unexpected fix result: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
			}
		})
	}
}

func TestE2EInventory_CompoundFixFlows(t *testing.T) {
	env := newE2EEnv(t)

	tests := []struct {
		name    string
		command string
		want    string
	}{
		{
			name:    "builtin and tool chain",
			command: "sourc ~/.zshrc && dcoker ps",
			want:    "source ~/.zshrc && docker ps\n",
		},
		{
			name:    "builtin and system pipeline",
			command: "echp ok | gerp ok && taill -n 1 app.log",
			want:    "echo ok | grep ok && tail -n 1 app.log\n",
		},
		{
			name:    "system wrapper and tool chain",
			command: "sudo maek test && crul https://example.com",
			want:    "sudo make test && curl https://example.com\n",
		},
		{
			name:    "command wrapper and tool command",
			command: "command gut status || terrafrom plan",
			want:    "command git status || terraform plan\n",
		},
		{
			name:    "mixed tool and builtin chain",
			command: `helmm list && evall "echo ok"`,
			want:    "helm list && eval \"echo ok\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := env.run(t, "fix", tt.command)
			if result.code != 0 || result.stdout != tt.want {
				t.Fatalf("unexpected fix result: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
			}
		})
	}
}
