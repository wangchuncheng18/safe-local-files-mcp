# Safe Local Files MCP

一个供本机 Codex 使用的文件 MCP 工具。每位使用者只授权自己电脑上的一个目录；默认只读，不扫描后台文件，不把 Token、配置或文件内容放进 GitHub 仓库。Windows、macOS 和 Linux 均可从源码构建，Release 提供对应平台的可执行文件。

> 本项目的 GitHub 仓库只分发程序。朋友安装后应选择**自己电脑**上的目录。仓库中没有、也不需要作者电脑的私有文件。

## 能做什么

默认提供 `search`、`fetch`、`list_directory`、`read_file`、`stat_path`。管理员可以独立开启创建文件、覆盖文件和创建目录。没有删除或移动工具。只有明确开启的写工具才会出现在 MCP 工具列表中。

路径都相对于配置的 `root`。程序拒绝目录穿越、默认拒绝符号链接与常见密钥文件，限制文件大小、搜索耗时、扫描数量和并发数。审计日志记录操作、相对路径、结果和耗时，不记录 Token 或文件内容。详见 [SECURITY.md](SECURITY.md) 和 [THREAT_MODEL.md](THREAT_MODEL.md)。

## 五分钟安装

需要 [Go](https://go.dev/dl/) 1.25 或更新版本、Codex CLI，以及 Git。克隆本仓库后，从仓库根目录运行：

Windows PowerShell：

```powershell
git clone https://github.com/wangchuncheng18/safe-local-files-mcp.git
cd safe-local-files-mcp
.\install.ps1
```

macOS / Linux：

```sh
git clone https://github.com/wangchuncheng18/safe-local-files-mcp.git
cd safe-local-files-mcp
./install.sh
```

安装器会编译程序、安装个人插件、建立私有配置和 Token，并把 `safe_local_files` 注册为 Codex 的 `stdio` MCP。Codex 在需要时自动启动服务，因此本机快速对话不依赖常驻端口。安装完成后，**先把配置里的 `root` 改成你自己的目录，再新建一个 Codex 对话**。

私有配置位置：

| 系统 | 配置文件 | 审计日志 |
| --- | --- | --- |
| Windows | `%LOCALAPPDATA%\SafeLocalFiles\config.json` | 同目录的 `audit.jsonl` |
| macOS / Linux | `${XDG_CONFIG_HOME:-$HOME/.config}/safe-local-files/config.json` | 同目录的 `audit.jsonl` |

例如 Windows 可设为 `D:\\Documents\\AI-Share`（JSON 中反斜杠需要写两次）；macOS 可设为 `/Users/你的用户名/Documents/AI-Share`。建议新建专用目录，只放愿意让 AI 读取的文件。**不要把整个磁盘、主目录或生产数据目录直接设为根目录。**

安装和验证的完整步骤见 [INSTALL_FOR_AGENTS.md](INSTALL_FOR_AGENTS.md)，适合交给 AI 助手逐步执行；面向朋友的说明见 [SHARE_GUIDE.md](SHARE_GUIDE.md)。

## 权限配置

默认配置如下，改完后新建 Codex 对话生效：

```json
"write_permissions": {
  "enabled": false,
  "create_files": false,
  "overwrite_files": false,
  "create_directories": false
}
```

要允许 AI **只创建新文本文件**，设 `enabled` 和 `create_files` 为 `true`。要覆盖已有文件，另设 `overwrite_files` 为 `true`。建目录由 `create_directories` 控制。写入最多 `max_write_bytes`（默认 1 MiB），只能写允许的扩展名和 UTF‑8 文本；疑似密钥内容会被拒绝。覆盖时还会检查现有文件能否安全读取。删除和移动始终不可用。

本机现有使用者的私有配置不会随仓库更新自动改变，升级后仍保持原来的只读状态。

常用私有配置字段：

| 字段 | 默认值 | 用途 |
| --- | --- | --- |
| `root` | 示例目录 | 唯一授权目录；须改成自己机器上的路径 |
| `port` | `47381` | 可选 HTTP 端口；不影响本机 `stdio` |
| `token_env` | `SAFE_LOCAL_FILES_TOKEN` | HTTP Token 的环境变量名称 |
| `audit_log` | 用户私有目录 | JSONL 审计日志位置 |
| `deny_globs` | 密钥、凭据等规则 | 拒绝读取和写入的路径 |
| `allow_extensions` | 常见文本扩展名 | 可读取或写入的文件类型 |
| `max_write_bytes` | `1048576` | 单次写入最多字节数 |
| `write_permissions` | 全部关闭 | 分别控制创建、覆盖和建目录 |
| `listen` / `allow_remote` | `127.0.0.1` / `false` | 仅可选 HTTP 使用；远程需显式打开 |
| `tls_cert_file` / `tls_key_file` | 空 | 非回环监听时必填 |
| `allow_remote_write` | `false` | 非回环监听的额外写入开关 |

完整默认值见 [config.example.json](plugins/safe-local-files/config.example.json)。

## Token 和端口

本机 `stdio` 模式通过 Codex 启动的进程管道通信，不监听端口，也不需要 HTTP Token。Token 专供可选的 HTTP 模式使用。安装器生成 32 字节随机 Token，将它保存到用户私有文件，并设置用户环境变量 `SAFE_LOCAL_FILES_TOKEN`；配置文件只写环境变量**名称**，不写 Token 值。Token 不会在安装输出或审计日志中显示。

HTTP 默认只监听 `127.0.0.1:47381`。可在私有 `config.json` 中改 `port`，然后重新启动 HTTP 服务；`stdio` 模式不受端口影响。需要轮换 HTTP Token 时运行：

```powershell
.\plugins\safe-local-files\scripts\rotate-token.ps1 -RestartHttp
```

```sh
./plugins/safe-local-files/scripts/rotate-token.sh --restart-http
```

轮换后旧 Token 失效；使用 HTTP 的客户端需要重新取得新值。不要把 Token 发给别人、写进仓库或贴进对话。朋友的安装器会为朋友生成独立 Token。

## 可选的 HTTP 服务和跨机器访问

需要其他本地客户端时运行 `scripts/start.ps1` 或 `scripts/start.sh`，停止用对应的 `stop` 脚本。HTTP MCP 路径为 `/mcp`，健康检查路径为 `/healthz`。跨机器访问须显式设置 `allow_remote: true`，监听指定 IP，提供 TLS 证书和私钥；如果同时开启写入，还须设置 `allow_remote_write: true`。建议通过私有 VPN 连接，并把监听地址限制为 VPN 网卡 IP。不要把端口直接映射到公网。部署方案和边界见 [REMOTE_ACCESS.md](REMOTE_ACCESS.md)。

云端 ChatGPT 网页会话无法直接访问你电脑的 `127.0.0.1`。本项目的本机安装目标是运行在使用者电脑上的 Codex/ChatGPT 桌面环境；跨机器场景需要由使用者自行建立可信网络连接。

## 验证和故障排查

```sh
codex mcp get safe_local_files
```

结果应显示 `enabled: true`、`transport: stdio`。新建对话后先试“使用 `safe_local_files.stat_path` 检查授权根目录 `.`，不要列出文件内容”。若无工具，检查是否新建了对话、配置里的根目录是否存在、可执行文件是否仍在安装位置。HTTP 模式再检查 Token 环境变量和端口。

源码检查：

```sh
cd plugins/safe-local-files
go test ./...
go vet ./...
```

## 发布包与许可证

[GitHub Releases](https://github.com/wangchuncheng18/safe-local-files-mcp/releases) 提供 Windows、macOS、Linux 的 AMD64/ARM64 包与 SHA-256 文件。Release 包可手动安装；仓库根目录安装脚本适合从源码一键安装。许可证为 [MIT](LICENSE)。
