# Safe Local Files MCP

一个用于本机和远程文件访问的 MCP 服务。每位使用者在自己的 Windows、macOS 或 Linux 机器上部署服务，并只授权该机器的一个目录。默认只读，不扫描后台文件，不把 Token、配置或文件内容放进 GitHub 仓库。

> 本项目的 GitHub 仓库只分发程序。朋友安装后应选择**自己电脑**上的目录。仓库中没有、也不需要作者电脑的私有文件。

**想让 GPT 快速聊天读取自己的机器？** 直接按 [QUICK_CHAT_SETUP.md](QUICK_CHAT_SETUP.md) 操作；其中有可以交给朋友的本地 Codex 的完整任务文本。每个人使用自己的授权目录、Cloudflare 域名和 ChatGPT MCP 连接。推荐主机名 `files.<自己的域名>`，Tunnel 名 `safe-local-files`。域名必须由部署者控制并接入 Cloudflare；给它取个名字本身不会生成公网地址。目前目标是只读，写入留待以后单独处理。

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

安装器会编译程序、安装个人插件、建立私有配置和 Token，并把 `safe_local_files` 注册为 Codex 的 `stdio` MCP。Codex 在需要时自动启动服务，因此本机 Codex 任务不依赖常驻端口。安装完成后，**先把配置里的 `root` 改成你自己的目录，再新建一个 Codex 任务**。这一步不会自动给云端 GPT 快速聊天注册工具；其接入方式见下文。

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
| `auth_mode` | `bearer_token` | HTTP 鉴权模式；Cloudflare Access 可改为 `cloudflare_access` |
| `cloudflare_team_domain` / `cloudflare_audience` | 空 | Cloudflare Access 模式的团队域名与应用 AUD tag |
| `audit_log` | 用户私有目录 | JSONL 审计日志位置 |
| `deny_globs` | 密钥、凭据等规则 | 拒绝读取和写入的路径 |
| `allow_extensions` | 常见文本扩展名 | 可读取或写入的文件类型 |
| `max_write_bytes` | `1048576` | 单次写入最多字节数 |
| `write_permissions` | 全部关闭 | 分别控制创建、覆盖和建目录 |
| `listen` / `allow_remote` | `127.0.0.1` / `false` | 仅可选 HTTP 使用；远程需显式打开 |
| `behind_proxy` | `false` | HTTPS 代理转发到本机回环地址时设为 `true`；写入还需独立开关 |
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

## 选择 MCP 地址与接入方式

**目录由服务端决定，地址由客户端选择。** 例如 `https://files.alice.example/mcp` 连接 Alice 机器上的服务与其授权目录，`https://files.bob.example/mcp` 连接 Bob 的机器与目录。MCP 请求中的相对路径不能切换主机或逃出该服务的 `root`。每人各自保存配置、Token 和审计日志。不要让用户在工具参数里输入任意 IP，让同一个服务充当开放式文件代理。

| 客户端与网络 | 推荐链路 |
| --- | --- |
| 本机 Codex 任务 | `stdio`，无需网络端口 |
| GPT 快速聊天访问自己的机器 | [Cloudflare Tunnel + Access](QUICK_CHAT_SETUP.md)：自己的稳定 HTTPS 域名 → Cloudflare Managed OAuth → 同机 `127.0.0.1:47381/mcp` |
| 其他私有隧道方案 | [Secure MCP Tunnel](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels)：可用于特定 ChatGPT 工作区，配置步骤与 Cloudflare 路线不同 |
| 内网或 VPN 内的 Codex 客户端 | 使用 VPN 地址的 HTTPS MCP；客户端保存自己的凭据 |

`127.0.0.1` 永远指当前发起连接的机器。内网 DNS、`hosts` 文件或 VPN IP 只对能进入该网络的客户端有效，不能让云端 GPT 快速聊天直接访问本机。完整链路、部署边界和 HTTPS 网关要求见 [REMOTE_ACCESS.md](REMOTE_ACCESS.md)。

## 可选的 HTTP 服务和跨机器访问

需要其他本地客户端时运行 `scripts/start.ps1` 或 `scripts/start.sh`，停止用对应的 `stop` 脚本。HTTP MCP 路径为 `/mcp`，健康检查路径为 `/healthz`。跨机器访问须显式设置 `allow_remote: true`，监听指定 IP，提供 TLS 证书和私钥；如果同时开启写入，还须设置 `allow_remote_write: true`。建议通过私有 VPN 连接，并把监听地址限制为 VPN 网卡 IP。不要把端口直接映射到公网。部署方案和边界见 [REMOTE_ACCESS.md](REMOTE_ACCESS.md)。

GPT 快速聊天即使在桌面 App 中打开，MCP 连接仍须在 ChatGPT 一侧注册。仅安装本地插件或在 `hosts` 文件中添加名称不会使快速聊天获得本机 MCP 工具。ChatGPT 不能呈现自定义静态 API key。Cloudflare Access 模式通过 Managed OAuth 让客户端登录，并由本服务验证 `Cf-Access-Jwt-Assertion`；原有静态 Bearer Token 只用于其他私有 HTTP 客户端。[OpenAI 认证要求](https://developers.openai.com/plugins/build/auth)、[Cloudflare Managed OAuth](https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/managed-oauth/)

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
