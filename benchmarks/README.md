# Typo Benchmarks

This directory contains the benchmark suite for `typo`.

## Contents

```text
benchmarks/
|- bench.sh              # Convenience wrapper for the full benchmark run
|- benchmark_test.go     # End-to-end CLI benchmarks
|- engine_bench_test.go  # In-process engine and utility benchmarks
`- README.md             # This document
```

## What Is Covered

- CLI benchmarks that spawn the `typo` executable and measure full process cost.
- Engine benchmarks that measure the hot path inside the correction engine.
- Utility benchmarks for distance, similarity, rule lookup, history lookup, and keyboard adjacency checks.

## Prerequisites

- Go `1.25+`
- A runnable `typo` binary available on `PATH` for `BenchmarkTypoCLI`

The CLI benchmarks call `exec.Command("typo", ...)`, so they measure process startup, argument parsing, and the correction flow together.

## How To Run

Run the full suite:

```bash
./benchmarks/bench.sh
```

Run the same command directly:

```bash
go test -bench=. -benchmem ./benchmarks/ -run=^$ -count=1
```

Run only the CLI benchmarks:

```bash
go test -bench=BenchmarkTypoCLI -benchmem ./benchmarks/ -run=^$ -count=1
```

## Latest Sample Report

Date: `2026-03-25`

Environment:

- OS: `darwin/arm64`
- CPU: `Apple M4`
- Go: `go1.25.8`
- Command: `go test -bench=. -benchmem ./benchmarks/ -run=^$ -count=1`

### CLI Benchmarks

| Benchmark | Time | Memory | Allocs |
| --- | ---: | ---: | ---: |
| `fix-rule-match` | `28.11 ms/op` | `18528 B/op` | `47 allocs/op` |
| `fix-distance-match` | `32.58 ms/op` | `18528 B/op` | `47 allocs/op` |
| `fix-no-match` | `30.75 ms/op` | `18552 B/op` | `48 allocs/op` |
| `rules-list` | `28.32 ms/op` | `18520 B/op` | `47 allocs/op` |
| `version` | `11.41 ms/op` | `18496 B/op` | `47 allocs/op` |

### Distance Benchmarks

| Benchmark | Time | Memory | Allocs |
| --- | ---: | ---: | ---: |
| `short-same` | `128.9 ns/op` | `224 B/op` | `5 allocs/op` |
| `short-close` | `134.9 ns/op` | `224 B/op` | `5 allocs/op` |
| `short-far` | `144.2 ns/op` | `224 B/op` | `5 allocs/op` |
| `medium-same` | `409.3 ns/op` | `624 B/op` | `8 allocs/op` |
| `medium-close` | `414.8 ns/op` | `624 B/op` | `8 allocs/op` |
| `long-same` | `1115 ns/op` | `1344 B/op` | `12 allocs/op` |
| `long-close` | `987.3 ns/op` | `1200 B/op` | `11 allocs/op` |

### Similarity Benchmarks

| Benchmark | Time | Memory | Allocs |
| --- | ---: | ---: | ---: |
| `short` | `139.4 ns/op` | `224 B/op` | `5 allocs/op` |
| `medium` | `418.9 ns/op` | `624 B/op` | `8 allocs/op` |
| `long` | `990.9 ns/op` | `1200 B/op` | `11 allocs/op` |

### Engine Benchmarks

| Benchmark | Time | Memory | Allocs |
| --- | ---: | ---: | ---: |
| `exact-match-rule` | `112.3 ns/op` | `80 B/op` | `3 allocs/op` |
| `distance-match` | `113.8 ns/op` | `80 B/op` | `3 allocs/op` |
| `no-match` | `3837 ns/op` | `4776 B/op` | `83 allocs/op` |

### Core Utility Benchmarks

| Benchmark | Time | Memory | Allocs |
| --- | ---: | ---: | ---: |
| `Keyboard.IsAdjacent` | `18.06 ns/op` | `0 B/op` | `0 allocs/op` |
| `Rules.Match` | `41.69 ns/op` | `0 B/op` | `0 allocs/op` |
| `History.Lookup` | `22.60 ns/op` | `0 B/op` | `0 allocs/op` |

### Large Command Set Benchmarks

| Benchmark | Time | Memory | Allocs |
| --- | ---: | ---: | ---: |
| `distance-short` | `9653 ns/op` | `12928 B/op` | `244 allocs/op` |
| `distance-medium` | `21185 ns/op` | `24976 B/op` | `424 allocs/op` |

## Notes

- In-process correction is sub-`150 ns/op` for the fast match paths in the current dataset.
- No-match and large-command-set cases move into the low-microsecond range because more candidates must be scanned.
- CLI benchmarks are much slower than engine benchmarks because they include process startup and command execution overhead.
- Sample numbers depend on the machine, Go version, CPU state, and whether the binary and filesystem caches are warm.
