#!/usr/bin/env python3
"""Reproduce audit findings using isolated homes and an existing typo binary."""

import argparse
import concurrent.futures
import json
import os
from pathlib import Path
import select
import shutil
import subprocess
import tempfile
import time


ROOT = Path(__file__).resolve().parents[2]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    binary = args.binary.resolve()
    if not binary.is_file():
        parser.error("build the typo binary first")
    args.output.parent.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="audit-repro-", dir=args.output.parent))
    results = []

    def environment(name):
        case = work / name
        for part in ("home", "tmp"):
            (case / part).mkdir(parents=True)
        return case, {
            "HOME": str(case / "home"),
            "TMPDIR": str(case / "tmp"),
            "PATH": "/usr/bin:/bin:/opt/homebrew/bin",
            "LC_ALL": "C",
        }

    def run(argv, env):
        completed = subprocess.run(
            argv, cwd=ROOT, env=env, capture_output=True, text=True, timeout=15
        )
        return {
            "code": completed.returncode,
            "stdout": completed.stdout,
            "stderr": completed.stderr,
        }

    def record(finding, name, healthy, evidence):
        results.append({
            "finding": finding,
            "name": name,
            "status": "PASS" if healthy else "ISSUE",
            "evidence": evidence,
        })

    # Serial controls and parallel writers use different data directories.
    for kind in ("rules", "history"):
        for workers in (1, 16):
            case, env = environment(f"{kind}-{workers}")

            def write_one(index):
                if kind == "rules":
                    argv = [binary, "learn", f"auditword{index}", "git status"]
                else:
                    argv = [binary, "fix", f"gti status -- audit{index}"]
                return run(argv, env)

            with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as pool:
                writes = list(pool.map(write_one, range(40)))
            filename = "rules.json" if kind == "rules" else "usage_history.json"
            entries = json.loads((case / "home" / ".typo" / filename).read_text())
            successful = sum(item["code"] == 0 for item in writes)
            record("F01", f"{kind}-{workers}-writers", successful == len(entries) == 40, {
                "submitted": 40, "successful": successful, "persisted": len(entries)
            })

    # Measure the first prompt marker, not process completion: a background
    # process can keep capture pipes open after the prompt is already available.
    bash_paths = list(dict.fromkeys(filter(None, ["/bin/bash", shutil.which("bash")])))
    for shell_index, bash in enumerate(bash_paths):
        for integrated in (False, True):
            case, env = environment(f"background-{shell_index}-{integrated}")
            code = 'source "$1"; trap - DEBUG; _typo_preexec;\n' if integrated else ""
            code += "SECONDS=0; sleep 2 &\n"
            if integrated:
                code += "_typo_precmd;\n"
            code += 'printf "prompt_wait_seconds=%s\\n" "$SECONDS"'
            process = subprocess.Popen(
                [bash, "--noprofile", "--norc", "-c", code, "bash", str(ROOT / "install/typo.bash")],
                env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
            )
            started = time.monotonic()
            ready, _, _ = select.select([process.stdout], [], [], 8)
            if not ready:
                process.kill()
                process.communicate()
                raise RuntimeError("background probe did not produce a prompt marker")
            marker = process.stdout.readline()
            elapsed = time.monotonic() - started
            _, stderr = process.communicate(timeout=8)
            record("F02", f"background-{bash}-{integrated}", elapsed < 1, {
                "shell": bash, "integration": integrated,
                "prompt_latency_seconds": round(elapsed, 3), "marker": marker,
                "code": process.returncode, "stderr": stderr,
            })

            for kind, expected in (("exit", "status=7\n"), ("prompt", "status=1\n")):
                case, env = environment(f"status-{shell_index}-{integrated}-{kind}")
                if kind == "exit":
                    code = 'trap \'printf "status=%s\\n" "$?"\' EXIT\n'
                else:
                    code = 'PROMPT_COMMAND=\'printf "status=%s\\n" "$?"\'\n'
                if integrated:
                    code += 'source "$1"; trap - DEBUG;\n'
                code += 'exit 7' if kind == "exit" else 'false; eval "$PROMPT_COMMAND"'
                result = run(
                    [bash, "--noprofile", "--norc", "-c", code, "bash", str(ROOT / "install/typo.bash")],
                    env,
                )
                record("F04", f"{kind}-{bash}-{integrated}", result["stdout"] == expected, {
                    "expected_stdout": expected, **result
                })

    case, env = environment("local-variables")
    context = case / "context.tsv"
    context.write_text("bash\tenv\tHOME\tHOME\n", encoding="utf-8")
    for command in (
        'HOEM=/tmp; echo "$HOEM"',
        'for HOEM in /tmp; do echo "$HOEM"; done',
    ):
        result = run([binary, "fix", "--no-history", "--alias-context", context, command], env)
        original = run(["/bin/bash", "-c", command], env)
        corrected = run(["/bin/bash", "-c", result["stdout"]], env) if result["code"] == 0 else original
        record("F03", command, original["stdout"] == corrected["stdout"], {
            "input": command, "fix": result,
            "original_stdout": original["stdout"], "corrected_stdout": corrected["stdout"],
        })
    control = run([binary, "fix", "--no-history", "--alias-context", context, "echo $HOEM"], env)
    record("F03", "undefined-variable-control", control["stdout"] == "echo $HOME\n", control)

    # Two malformed versions must leave two recoverable artifacts. Wait until
    # the start of a second to make the existing second-resolution collision reproducible.
    case, env = environment("quarantine")
    config_dir = case / "home/.typo"
    config_dir.mkdir()
    time.sleep(1.02 - time.time() % 1)
    attempts = []
    for index in range(2):
        (config_dir / "config.json").write_text(f'{{"audit_bad_{index}":', encoding="utf-8")
        attempts.append(run([binary, "config", "list"], env))
    backups = {p.name: p.read_text() for p in config_dir.glob("config.json.corrupt-*")}
    record("F05", "quarantine-preserves-both-versions", len(backups) == 2, {
        "attempts": attempts, "backups": backups,
    })

    report = {"work_directory": str(work), "results": results}
    args.output.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    for result in results:
        print(f'{result["status"]} {result["finding"]} {result["name"]}')
    raise SystemExit(1 if any(r["status"] == "ISSUE" for r in results) else 0)


if __name__ == "__main__":
    main()
