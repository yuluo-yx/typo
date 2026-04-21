# 工作原理

[English](how-it-works.md) | 简体中文

## 修正优先级

Typo 按以下顺序修正命令：

1. 错误解析
2. 用户规则
3. 历史记录
4. 内置规则
5. 子命令修正
6. 编辑距离匹配

这意味着真实报错信息和用户显式覆盖会先于模糊匹配生效。

## 错误解析

当存在真实 `stderr` 输出时，Typo 会优先从报错信息中提取建议。

当前已有文档覆盖的解析器包括：

- `git`：`did you mean...`、缺少 upstream 等建议
- `docker`：未知命令建议
- `npm`：命令未找到建议

zsh、bash 和 PowerShell 集成可以自动通过 `typo fix -s <file>` 传入这份 `stderr` 缓存。
在 PowerShell 中，当前首个受支持版本对 native command 的这条链路最稳定。
fish 集成支持当前缓冲区修正，以及空缓冲区时结合上一条历史命令和退出码进行修正；首个受支持版本不会创建或传入 `typo-stderr-*` 实时 `stderr` 缓存。

示例：

```bash
typo fix -s git.stderr "git remove -v"
typo fix -s git.stderr "git pull"
typo fix -s docker.stderr "docker psa"
typo fix -s npm.stderr "npm isntall react"
```

## 子命令修正

Typo 不仅能修正顶层命令，也能修正常见工具的子命令。

常见支持工具包括：

- `git`、`docker`、`npm`、`yarn`、`kubectl`、`cargo`、`go`
- `pip`、`brew`、`terraform`、`helm`
- `aws`、`sam`、`cdk`、`eksctl`、`gcloud`、`gsutil`、`az`、`func`、`azd`、`doctl`、`oci`、`linode-cli` 等云平台 CLI

补充说明：

- 常用云工具会作为内置命令候选加入，即使 PATH 发现还没执行，也能先修正顶层命令。
- `git`、`docker`、`kubectl` 内置了树形子命令结构，即使动态发现还没执行，也能修正常见多层子命令。
- 动态发现到的子命令会以 `schema_version: 2` 的树形结构缓存到 `~/.typo/subcommands.json`。
- `git`、`docker`、`aws`、`gcloud`、`az` 支持层级化子命令发现；`kubectl` 资源修正使用保守的内置资源树。
- 不符合 `schema_version: 2` 的旧子命令缓存会被自动隔离，并在后续使用时重新生成。

### 子命令缓存 v2

`subcommands.json` 使用带版本号的树形格式：

```json
{
  "schema_version": 2,
  "tools": [
    {
      "tool": "git",
      "tree": {
        "children": {
          "stash": {
            "children": {
              "save": {"terminal": true}
            }
          }
        }
      },
      "updated_at": "2026-04-15T12:00:00Z"
    }
  ]
}
```

v2 会直接保存嵌套子命令，不再使用“根级扁平列表 + 路径子列表”的旧格式。这样 Typo 可以逐层修正命令，例如把 `gcloud container clusers lisr` 修正为 `gcloud container clusters list`。

节点字段含义如下：

- `children`：当前层级下的有效子命令。
- `terminal`：该 token 可以作为命令路径的结束。
- `passthrough`：该 token 后面的内容按用户参数处理，不再继续当作子命令修正。
- `alias`：短别名对应的规范输出名称。

如果 Typo 发现旧格式缓存，会把该文件重命名为 `subcommands.json.corrupt-<时间戳>`，并在下次使用时重新生成 v2 缓存。该文件只包含动态发现到的命令元数据，不包含用户规则、历史记录或配置，因此隔离旧缓存不会丢失用户数据。

## 本地文件

Typo 会把本地状态保存在 `~/.typo/`：

```text
~/.typo/
├── config.json
├── rules.json
├── usage_history.json
└── subcommands.json
```

各文件作用：

- `config.json`：由 `typo config` 管理的运行配置
- `rules.json`：learn 结果和用户自定义规则
- `usage_history.json`：已接受修正的历史记录；当重复 pair 被提升为用户规则后，条目会带上 `rule_applied: true`
- `subcommands.json`：动态发现到的子命令树缓存

## 配置模型

当前默认暴露的配置项包括：

- `similarity-threshold`
- `max-edit-distance`
- `max-fix-passes`
- `auto-learn-threshold`
- `keyboard`
- `history.enabled`
- `rules.<scope>.enabled`

支持的键盘布局：

- `qwerty`
- `dvorak`
- `colemak`

内置规则作用域：

- `git`、`docker`、`npm`、`yarn`、`kubectl`、`cargo`、`brew`
- `helm`、`terraform`、`python`、`pip`、`go`、`java`、`system`

## 构建与验证

本仓库推荐使用 Makefile 目标进行本地开发：

```bash
make build
make build-all
make install
make test
make coverage
make lint
```

文档相关检查：

- `make markdown-lint`：Markdown 文档检查
- `make codespell-check`：拼写检查

## 相关文档

- CLI 用法请看 [命令参考](commands_CN.md)。
- v1.x 稳定性承诺请看 [稳定性契约](stability_CN.md)。
- 安装说明请看 [快速开始](../getting-started/quick-start_CN.md)。
- 用户视角的修正场景请看 [使用示例](../example/use_CN.md)。
