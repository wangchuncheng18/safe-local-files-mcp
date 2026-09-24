# 把 Safe Local Files 分享给朋友

把此仓库链接发给朋友即可：[GitHub 项目](https://github.com/wangchuncheng18/safe-local-files-mcp)。程序不会把你的电脑目录共享给朋友。朋友安装后在**自己的电脑**选择一个目录，拥有独立的私有配置和 Token。

## 朋友需要准备什么

- Windows、macOS 或 Linux 电脑；已安装 Git、Go 1.25+ 和 Codex CLI。
- 一个专门给 AI 使用的目录。建议先放少量不敏感的文本文件进行测试。
- 安装时能运行 PowerShell（Windows）或终端（macOS/Linux）。

Windows 在仓库根目录运行 `.\install.ps1`；macOS/Linux 运行 `./install.sh`。安装脚本会编译适合本机的程序并注册 `safe_local_files`。详细命令和配置文件位置见 [README.md](README.md) 或 [INSTALL_FOR_AGENTS.md](INSTALL_FOR_AGENTS.md)。也可以把后者交给 AI 助手按步骤操作。

安装后必须打开自己的私有 `config.json`，把 `root` 改成自己的目录；保持写权限全关，先新建 Codex 对话试：

> 使用 `safe_local_files.stat_path` 检查授权根目录 `.`，只告诉我它是不是目录，不读取文件。

成功后可试：

> 使用 Safe Local Files 在授权目录搜索“会议纪要”，最多返回 5 条，不展示疑似密钥内容。

本地 Codex 使用 `stdio`，由对话自动启动 MCP 进程，不需要把电脑端口暴露到网络。HTTP 端口默认是 `47381`，仅在需要额外 HTTP 客户端时使用。不同电脑可选不同端口，朋友之间也不需要共享 Token。

## 如果朋友想让 AI 写文件

默认只读。只有朋友在自己的私有配置中明确打开 `write_permissions`，AI 才看得到写工具。可以先只开启新建文件，之后再按需开启覆盖或建目录。程序没有删除或移动工具；每次写入仍受目录、文件类型、大小和敏感内容规则限制。建议先用一个空的测试目录练习。

## 常见问题

**安装了插件但 GPT 快速聊天无反应？** `codex mcp get safe_local_files` 只检查 Codex 本机任务的连接，不代表 GPT 快速聊天也能调用它。快速聊天须另外注册可达的 Secure MCP Tunnel 或 HTTPS MCP 连接，并在聊天中启用该连接。仅看到插件技能名称并不能证明 MCP 工具已经连接。

**为什么朋友不能访问我的文件？** 每台电脑的 `root` 都是本地私有配置，默认没有跨机器连接。仓库不包含任何人的私有配置或文件。

**能从另一台电脑或 GPT 快速聊天访问吗？** 可以。每位朋友在自己的机器部署服务，并为该机器选择私有隧道或 HTTPS 地址；连接地址决定访问的是哪台机器的授权目录。仅改 `hosts` 文件不能让云端 GPT 快速聊天进入内网。请先阅读 [REMOTE_ACCESS.md](REMOTE_ACCESS.md)。不建议直接做路由器公网端口映射。

**可以用 cloudflared 吗？** 可以，推荐 Named Tunnel 配合 Cloudflare Access Managed OAuth。每位朋友需要自己的 Cloudflare 域名、Zero Trust 团队、仅允许自己登录的 Access 策略和 Tunnel。服务端须设为 `cloudflare_access` 模式并验证 Access JWT。无域名的随机 Quick Tunnel 不适合长期插件地址。完整步骤见 [CLOUDFLARE.md](CLOUDFLARE.md)。
