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
		{name: "type builtin transposition avoids fuzzy mismatch", command: "tyep git", want: "type git\n"},
		{name: "hash builtin", command: "hasj git", want: "hash git\n"},
		{name: "help builtin", command: "helpp git", want: "help git\n"},
		{name: "test builtin", command: "testt -f file", want: "test -f file\n"},
		{name: "true builtin", command: "truee", want: "true\n"},
		{name: "false builtin", command: "falsee", want: "false\n"},
		{name: "eval builtin", command: `evall "echo ok"`, want: "eval \"echo ok\"\n"},
		{name: "exec builtin", command: "execc ls", want: "exec ls\n"},
		{name: "ls command", command: "sl -la", want: "ls -la\n"},
		{name: "cd command", command: "dc /tmp", want: "cd /tmp\n"},
		{name: "pwd command", command: "pdw", want: "pwd\n"},
		{name: "cat command", command: "cta /tmp/file", want: "cat /tmp/file\n"},
		{name: "grep command", command: "gerp main app.log", want: "grep main app.log\n"},
		{name: "tail command", command: "taill -n 1 app.log", want: "tail -n 1 app.log\n"},
		{name: "head command", command: "headd -n 1 app.log", want: "head -n 1 app.log\n"},
		{name: "sort command", command: "srot data.txt", want: "sort data.txt\n"},
		{name: "uniq command", command: "unqi data.txt", want: "uniq data.txt\n"},
		{name: "cut command", command: "ctu -d: -f1 /etc/passwd", want: "cut -d: -f1 /etc/passwd\n"},
		{name: "tee command", command: "teee out.log", want: "tee out.log\n"},
		{name: "wc command", command: "wcc -l file.txt", want: "wc -l file.txt\n"},
		//nolint:misspell
		{name: "which command", command: "whcih git", want: "which git\n"},
		{name: "less command", command: "lesss README.md", want: "less README.md\n"},
		{name: "mkdir command", command: "mkdi tmpdir", want: "mkdir tmpdir\n"},
		{name: "rm command", command: "rmm file.txt", want: "rm file.txt\n"},
		{name: "cp command", command: "cpx a.txt b.txt", want: "cp a.txt b.txt\n"},
		{name: "mv command", command: "mvv a.txt b.txt", want: "mv a.txt b.txt\n"},
		{name: "touch command", command: "touc file.txt", want: "touch file.txt\n"},
		{name: "find command", command: "fin . -name main.go", want: "find . -name main.go\n"},
		{name: "sed command", command: "sedd -n 1p file.txt", want: "sed -n 1p file.txt\n"},
		{name: "awk command", command: `awkk "{print $1}" file.txt`, want: "awk \"{print $1}\" file.txt\n"},
		{name: "xargs command", command: "xagrs rm", want: "xargs rm\n"},
		{name: "sudo command", command: "sduo ls", want: "sudo ls\n"},
		{name: "make command", command: "maek test", want: "make test\n"},
		{name: "curl command", command: "crul https://example.com", want: "curl https://example.com\n"},
		{name: "tar command", command: "tra -tf archive.tar", want: "tar -tf archive.tar\n"},
		{name: "chmod command", command: "chmdo 755 deploy.sh", want: "chmod 755 deploy.sh\n"},
		{name: "chown command", command: "chownn root:staff deploy.sh", want: "chown root:staff deploy.sh\n"},
		{name: "ssh command", command: "sshh deploy@example.com", want: "ssh deploy@example.com\n"},
		{name: "scp command", command: "scpp build.tar.gz deploy@example.com:/srv/app/", want: "scp build.tar.gz deploy@example.com:/srv/app/\n"},
		{name: "wget command", command: "wgett https://example.com/tool.tgz", want: "wget https://example.com/tool.tgz\n"},
		{name: "wget transposition command", command: "wegt https://example.com/tool.tgz", want: "wget https://example.com/tool.tgz\n"},
		{name: "wget transposition variant command", command: "wgte https://example.com/tool.tgz", want: "wget https://example.com/tool.tgz\n"},
		{name: "rsync command", command: "rsycn -av dist/ backup/", want: "rsync -av dist/ backup/\n"},
		{name: "zip command", command: "zipp -r bundle.zip dist/", want: "zip -r bundle.zip dist/\n"},
		{name: "unzip command", command: "unzpi bundle.zip", want: "unzip bundle.zip\n"},
		{name: "python3 command", command: "python33 script.py", want: "python3 script.py\n"},
		{name: "node command", command: "nodee server.js", want: "node server.js\n"},
		{name: "su command", command: "suu deploy", want: "su deploy\n"},
		{name: "gzip command", command: "gzipp archive.tar", want: "gzip archive.tar\n"},
		{name: "gzip transposition command", command: "gizp archive.tar", want: "gzip archive.tar\n"},
		{name: "ps command", command: "pss aux", want: "ps aux\n"},
		{name: "kill command", command: "killl -9 1234", want: "kill -9 1234\n"},
		{name: "ln command", command: "lnn -s src dst", want: "ln -s src dst\n"},
		{name: "du command", command: "duu -sh .", want: "du -sh .\n"},
		{name: "df command", command: "dff -h", want: "df -h\n"},
		{name: "date command", command: "daet", want: "date\n"},
		//nolint:misspell
		{name: "open command", command: "opne .", want: "open .\n"},
		//nolint:misspell
		{name: "clear command", command: "claer", want: "clear\n"},
		{name: "man command", command: "mna ls", want: "man ls\n"},
		{name: "whoami command", command: "whomai", want: "whoami\n"},
		{name: "uname command", command: "unmae -a", want: "uname -a\n"},
		{name: "basename command", command: "basenmae /tmp/file.txt", want: "basename /tmp/file.txt\n"},
		{name: "dirname command", command: "dirnmae /tmp/file.txt", want: "dirname /tmp/file.txt\n"},
		{name: "file command", command: "fiel README.md", want: "file README.md\n"},
		{name: "stat command", command: "stta README.md", want: "stat README.md\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := env.run(t, "fix", "--no-history", tt.command)
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
		{name: "cargo help subcommand", command: "cargo helpd", want: "cargo help\n"},
		{name: "cargo global option", command: "cargo --versino", want: "cargo --version\n"},
		{name: "go main command", command: "og test ./...", want: "go test ./...\n"},
		{name: "go subcommand", command: "go biuld ./...", want: "go build ./...\n"},
		{name: "pip main command", command: "pi list", want: "pip list\n"},
		{name: "pip subcommand", command: "pip instlal typo", want: "pip install typo\n"},
		{name: "pip3 subcommand", command: "pip3 instlal typo", want: "pip3 install typo\n"},
		{name: "brew main command", command: "bre update", want: "brew update\n"},
		//nolint:misspell // Intentional typo for correction coverage.
		{name: "brew subcommand", command: "brew upgarde", want: "brew upgrade\n"},
		{name: "terraform main command", command: "terrafrom plan", want: "terraform plan\n"},
		{name: "terraform subcommand", command: "terraform valdiate", want: "terraform validate\n"},
		{name: "terraform subcommand after global option with value", command: "terraform -chdir infra valdiate", want: "terraform -chdir infra validate\n"},
		{name: "helm main command", command: "helmm list", want: "helm list\n"},
		{name: "helm subcommand", command: "helm temlpate chart", want: "helm template chart\n"},
		{name: "helm subcommand after global option with value", command: "helm --kube-context prod temlpate chart", want: "helm --kube-context prod template chart\n"},
		{name: "aws main command", command: "awss s3 ls", want: "aws s3 ls\n"},
		{name: "gcloud main command", command: "gclodu auth login", want: "gcloud auth login\n"},
		{name: "gcloud nested subcommand after interleaved option with value", command: "gcloud compute --zone us-east1 isntances listt", want: "gcloud compute --zone us-east1 instances list\n"}, //nolint:misspell
		{name: "az main command", command: "azz group list", want: "az group list\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := env.run(t, "fix", "--no-history", tt.command)
			if result.code != 0 || result.stdout != tt.want {
				t.Fatalf("unexpected fix result: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
			}
		})
	}
}

func TestE2EInventory_TypoCommandTree(t *testing.T) {
	env := newE2EEnv(t)

	tests := []struct {
		name    string
		command string
		want    string
	}{
		{name: "root typo prefers typo over type", command: "typ doctro", want: "typo doctor\n"},
		{name: "root transposition typo", command: "tpyo doctro", want: "typo doctor\n"},
		{name: "nested typo subcommands", command: "typo hsitory lsit", want: "typo history list\n"},
		{name: "rules second level typo", command: "typo rulse lset", want: "typo rules list\n"},
		{name: "init second level typo", command: "typo inti zsh", want: "typo init zsh\n"},
		{name: "config subcommand typo", command: "typo config gte keyboard", want: "typo config get keyboard\n"},
		{name: "free form fix payload preserved", command: "typo fxi gut status", want: "typo fix gut status\n"},
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
			command: "echp ok | gerp ok | xagrs rm && taill -n 1 app.log",
			want:    "echo ok | grep ok | xargs rm && tail -n 1 app.log\n",
		},
		{
			name: "shell utility chain",
			//nolint:misspell
			command: "pdw && whcih git && dff -h && daet",
			want:    "pwd && which git && df -h && date\n",
		},
		{
			name:    "text processing pipeline",
			command: "srot data.txt | unqi | teee out.log && ctu -d: -f1 /etc/passwd",
			want:    "sort data.txt | uniq | tee out.log && cut -d: -f1 /etc/passwd\n",
		},
		{
			name: "shell inspection chain",
			//nolint:misspell
			command: "claer && whomai && unmae -a && stta README.md",
			want:    "clear && whoami && uname -a && stat README.md\n",
		},
		{
			name:    "system wrapper and tool chain",
			command: "sudo maek test && crul https://example.com",
			want:    "sudo make test && curl https://example.com\n",
		},
		{
			name:    "env and time wrappers",
			command: "env --unset HOME gut status | time -p taill -n 1 app.log",
			want:    "env --unset HOME git status | time -p tail -n 1 app.log\n",
		},
		{
			name:    "remote copy and permission commands",
			command: "scpp build.tar.gz deploy@example.com:/srv/app/ && chmdo 755 deploy.sh",
			want:    "scp build.tar.gz deploy@example.com:/srv/app/ && chmod 755 deploy.sh\n",
		},
		{
			name:    "package install and download commands",
			command: "pip3 instlal typo && wgett https://example.com/tool.tgz",
			want:    "pip3 install typo && wget https://example.com/tool.tgz\n",
		},
		{
			name:    "transposition commands",
			command: "wgte https://example.com/tool.tgz && gizp archive.tar",
			want:    "wget https://example.com/tool.tgz && gzip archive.tar\n",
		},
		{
			name:    "runtime and process commands",
			command: "python33 script.py && nodee server.js && pss aux",
			want:    "python3 script.py && node server.js && ps aux\n",
		},
		{
			name:    "user switch and process stop commands",
			command: "suu deploy && killl -9 1234 && gzipp archive.tar",
			want:    "su deploy && kill -9 1234 && gzip archive.tar\n",
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
			result := env.run(t, "fix", "--no-history", tt.command)
			if result.code != 0 || result.stdout != tt.want {
				t.Fatalf("unexpected fix result: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
			}
		})
	}
}
