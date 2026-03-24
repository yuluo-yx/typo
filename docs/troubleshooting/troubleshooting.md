# 问题排查

## 1. 在 IDEA 终端中使用时，esc 键会跳出终端？

按照下图取消 esc 绑定即可。

![IDEA Config](imgs/idea.png)

## 2. Ghostty 终端模拟器 ssh 连接时运行失败且终端异常？

> 严格来说，这不是 typo 的问题，是 Ghostty 终端模拟器的问题。

- 输入重复；
- delete 变插入空格等。

原因是：ghostty 的 terminfo 问题，在服务器的 zsh 配置中加入 `export TERM=xterm-256color` 即可。

See: https://ghostty.org/docs/help/terminfo#ssh
