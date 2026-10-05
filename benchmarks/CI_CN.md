# 性能 CI 的比较与告警

性能工作流使用同一台 runner 和同一 Go 工具链比较两个提交。应用、依赖、基准或工具链输入未变化时，只记录当前基准，不生成回归告警。手动输入 `force_compare=true` 可以强制与 `HEAD^` 比较。

## 采样与判定

工作流为基线和当前提交创建两个干净的临时 worktree。采样固定 `GOMAXPROCS=4`，使用 `GOFLAGS=-buildvcs=false` 避免提交元数据影响构建。两个 worktree 的路径长度一致，减少 CLI 基准父进程因工作目录长度不同产生的内存统计差异。

两个版本先预热，再按 A/B、B/A 的顺序交替采样，每个版本采集 10 次。每次运行的顺序、开始时间、耗时、平台、Go 版本与构建参数保存在 `environment.json` 中。Go 测试、依赖下载或 benchstat 失败时，工作流直接失败，不把工具错误视为「无回归」。

初次比较中，耗时、内存或分配次数增加至少 10%，且 `p < 0.05` 的指标进入复测。复测只运行候选场景，并反转第一轮的起始顺序。只有同一场景的同一指标在两次比较中都满足门槛，才创建或更新性能 Issue，并使工作流失败。

交替采样与复测降低了固定顺序造成的误报风险，但无法消除共享 runner 的所有噪声。显著性表示两批测量存在差异，不等于确定了代码层面的原因。Issue 提供完整比较、复测结果和提交范围，具体原因仍需结合代码及本地复现判断。

## 基线状态

没有确认回归时，工作流将当前提交写入 `benchmark-state.env` 并保存缓存。确认回归时保留旧基线，让后续运行继续与回归前版本比较。下载、采样或比较失败时也不推进状态。

缓存继续使用 `sha=` 和 `updated_at=` 格式，无迁移，直接替换。无缓存或缓存提交不可用时，回退到 `HEAD^`；没有可用父提交时只运行当前基准。

## 本地验证

运行工具回归测试：

```bash
python3 -m unittest discover -s tools -p 'benchmark_ci_test.py' -v
```

在项目根目录安装工作流锁定的 benchstat 版本，再运行两个已提交版本的交替基准：

```bash
go install golang.org/x/perf/cmd/benchstat@406019bb8b6893dd1245d31bf511c719619bb5c9
python3 tools/benchmark_ci.py sample --baseline HEAD^ --current HEAD --output-dir benchmark-results/initial
benchstat benchmark-results/initial/baseline.txt benchmark-results/initial/current.txt > benchmark-results/initial/comparison.txt
python3 tools/benchmark_ci.py detect --initial benchmark-results/initial/comparison.txt --report benchmark-results/candidates.txt --output benchmark-results/outputs.env
```

`sample` 只采样提交中的内容，不包含当前工作区未提交的修改。`--samples` 必须为不小于 6 的偶数；默认每个版本 10 次、每个场景每次约 1 秒。冒烟验证可以使用 `--samples 6 --benchtime 1x`，但这种短样本不能用于确认 Issue 中的性能回归。

如果候选输出中的 `regression=true`，使用其中的 `benchmark_pattern` 作为 `sample --pattern` 参数，加上 `--reverse`，将结果保存到 `benchmark-results/confirmation`。再次运行 benchstat 后，向 `detect` 传入 `--confirmation benchmark-results/confirmation/comparison.txt`，只保留两次比较共同满足门槛的指标。

本地验证不需要触发 GitHub 工作流。回滚时恢复 `.github/workflows/benchmark.yml`，并移除本次新增的 Python 工具、测试和本说明；基线缓存格式保持兼容。
