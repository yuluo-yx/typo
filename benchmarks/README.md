# Typo 性能测试

## 目录结构

```
benchmarks/
├── bench.sh             # 性能测试脚本
├── benchmark_test.go    # CLI 完整调用性能测试
├── engine_bench_test.go # 引擎核心性能测试
└── README.md            # 本文档
```

## 测试环境

- **OS**: macOS (darwin/arm64)
- **CPU**: Apple M4
- **Go**: 1.21+

## 运行测试

```bash
./benchmarks/bench.sh
```

或者单独运行：

```bash
go test -bench=. -benchmem ./internal/engine/ -run=^$
```

## 性能结果

### 编辑距离算法 (Levenshtein Distance)

| 测试 | 耗时 | 内存分配 |
|------|------|---------|
| short-same (git/git) | ~140 ns/op | 224 B/op, 5 allocs |
| short-close (gut/git) | ~144 ns/op | 224 B/op, 5 allocs |
| medium-same (docker/docker) | ~419 ns/op | 624 B/op, 8 allocs |
| medium-close (dcoker/docker) | ~421 ns/op | 624 B/op, 8 allocs |
| long-same (kubernetes/kubernetes) | ~1155 ns/op | 1344 B/op, 12 allocs |
| long-close (kubernete/kubernetes) | ~1005 ns/op | 1200 B/op, 11 allocs |

### 相似度计算

| 测试 | 耗时 | 内存分配 |
|------|------|---------|
| short (gut/git) | ~150 ns/op | 224 B/op, 5 allocs |
| medium (dcoker/docker) | ~436 ns/op | 624 B/op, 8 allocs |
| long (kubernete/kubernetes) | ~998 ns/op | 1200 B/op, 11 allocs |

### 修正引擎 (Engine.Fix)

| 场景 | 耗时 | 内存分配 |
|------|------|---------|
| 规则匹配命中 | **~106 ns/op** | 80 B/op, 3 allocs |
| 编辑距离匹配 | **~106 ns/op** | 80 B/op, 3 allocs |
| 无匹配 | ~3843 ns/op | 4776 B/op, 83 allocs |

### 核心组件

| 组件 | 耗时 | 内存分配 |
|------|------|---------|
| 键盘相邻检测 (IsAdjacent) | **~20 ns/op** | 0 B/op, 0 allocs |
| 规则匹配 (Rules.Match) | **~52 ns/op** | 0 B/op, 0 allocs |
| 历史查询 (History.Lookup) | **~24 ns/op** | 0 B/op, 0 allocs |

### 大命令集测试 (50+ 命令)

| 测试 | 耗时 | 内存分配 |
|------|------|---------|
| distance-short (gt) | ~9.5 µs/op | 12928 B/op, 244 allocs |
| distance-medium (dkcer) | ~21.7 µs/op | 24976 B/op, 424 allocs |

## 性能分析

### 优势

1. **规则匹配极快**: ~106 ns，适合高频调用
2. **历史查询极快**: ~24 ns，基于 map 实现
3. **键盘感知零开销**: 相邻键检测仅需 ~20 ns

### 瓶颈

1. **无匹配场景**: 需要遍历所有已知命令计算距离
2. **大命令集**: 命令越多，距离计算越多

### 优化建议

1. **缓存已知命令的距离结果**: 对于相同命令词避免重复计算
2. **并行计算距离**: 对大命令集使用 goroutine 并行计算
3. **预计算热门命令**: 缓存最常使用的修正结果

## 实际使用性能

对于典型的命令修正场景：

```
gut status -> git status    # ~106 ns (规则匹配)
gti commit -> git commit    # ~106 ns (规则匹配)
dkcer ps -> docker ps       # ~21 µs (50命令集的距离计算)
```

**结论**: 对于最常见的规则匹配场景，typo 可以在 100 纳秒内完成修正，完全满足 CLI 实时响应的需求。

## CLI 完整调用性能

测试完整 CLI 进程调用（包含进程启动开销）：

| 命令 | 耗时 |
|------|------|
| `typo fix gut status` | ~8.4 ms |
| `typo fix dkcer ps` | ~10 ms |
| `typo fix xyzabc` (无匹配) | ~9.8 ms |
| `typo rules list` | ~9.4 ms |
| `typo version` | ~2.4 ms |

**注意**: CLI 调用包含进程启动开销（约 2-3ms），核心修正逻辑仅需 100ns-10µs。
