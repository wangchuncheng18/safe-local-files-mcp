# 给 AI 助手的安装与配置步骤

本文件是一份可执行的安装清单。执行前确定数据机器的操作系统、**该机器**授权目录的绝对路径、期望的写权限，以及客户端是 Codex 任务还是 GPT 快速聊天。默认使用只读。不要把作者电脑的路径复制到另一台机器。

若用户已经有 Release 包而不想安装 Go，可使用对应系统和架构的归档并校验 SHA-256；从源码安装时，根目录 `install.ps1` / `install.sh` 会调用 `scripts/build.ps1` / `scripts/build.sh`。单独构建命令为 Windows 的 `.\plugins\safe-local-files\scripts\build.ps1`，以及 macOS/Linux 的 `./plugins/safe-local-files/scripts/build.sh`。归档中的程序位于 `bin/`，安装脚本则位于 GitHub 仓库根目录。

## 任务约束

1. 每位用户在自己的数据机器上安装一个实例；本机 Codex 使用 `stdio` MCP。默认不开放 HTTP 端口。
2. 用户未明确要求写入时，保持 `write_permissions.enabled=false`。即使用户说“可以写”，也分别明确创建、覆盖和建目录的授权范围；不提供删除能力。
3. 不读取、打印、复制或提交 Token、私有配置、审计日志及授权目录中的文件内容来验证安装。可用 `stat_path(".")` 验证。
4. 不修改系统防火墙、路由器端口映射、VPN 或 TLS 设置，除非用户明确要求跨机器访问。
5. 公开仓库只有程序代码；每台机器的 `root` 和 Token 独立配置。
6. 不要声称安装个人插件或 `codex mcp add` 会让 GPT 快速聊天得到工具。快速聊天必须单独注册可达的 MCP 连接。内网 `host`、私有 IP 和本机 `127.0.0.1` 对云端客户端不可直连。

## Windows

前置条件：Git、Go 1.25+、Codex CLI。PowerShell 在仓库根目录执行：

```powershell
git clone https://github.com/wangchuncheng18/safe-local-files-mcp.git
cd safe-local-files-mcp
.\install.ps1
codex mcp get safe_local_files
```

私有配置在 `%LOCALAPPDATA%\SafeLocalFiles\config.json`。用 JSON 编辑器把 `root` 改成用户指定的**存在的**绝对路径，例如 `D:\\AI-Share`。保存后执行：

```powershell
.\plugins\safe-local-files\bin\safe-local-files.exe validate --config "$env:LOCALAPPDATA\SafeLocalFiles\config.json"
```

`validate` 的 HTTP Token 由安装器写入用户环境变量；若当前 PowerShell 尚未继承它，可新开 PowerShell 后执行。`stdio` 直接启动时不要求 HTTP Token。修改配置后，新建 Codex 对话以加载新的工具列表。

## macOS / Linux

前置条件：Git、Go 1.25+、Codex CLI。终端在仓库根目录执行：

```sh
git clone https://github.com/wangchuncheng18/safe-local-files-mcp.git
cd safe-local-files-mcp
./install.sh
codex mcp get safe_local_files
```

私有配置在 `${XDG_CONFIG_HOME:-$HOME/.config}/safe-local-files/config.json`。把 `root` 改成用户指定的存在的绝对路径，例如 `/Users/alice/Documents/AI-Share` 或 `/home/alice/AI-Share`。运行：

```sh
./plugins/safe-local-files/bin/safe-local-files stdio --config "${XDG_CONFIG_HOME:-$HOME/.config}/safe-local-files/config.json"
```

该命令进入 MCP 管道等待客户端输入；人工检查时可按 Ctrl+C 退出。日常使用无需手动执行，因为 Codex 自动启动它。修改配置后，新建 Codex 对话。

## 配置写权限

在私有 `config.json` 中保留或加入：

```json
"max_write_bytes": 1048576,
"write_permissions": {
  "enabled": false,
  "create_files": false,
  "overwrite_files": false,
  "create_directories": false
}
```

根据用户明确授权，仅把对应项改为 `true`。例如只允许新建文件：`enabled=true` 和 `create_files=true`。所有写操作仅作用于 `root` 内允许的 UTF‑8 文本文件；禁止的路径、敏感内容和超限文件会被拒绝。覆盖不会自动创建不存在的文件。没有删除或移动工具。

如果 Codex 工具审批策略可配置，建议写工具保持逐次确认。请勿把用户的真实根目录或 Token 写回这个公共仓库。

## Token 与端口

`stdio` 不监听端口。HTTP 默认 `127.0.0.1:47381`；只有用户需要其他 HTTP 客户端时才启动后台服务。安装器在用户私有目录生成 Token，Token 值仅存于用户环境变量及权限受限的 `token` 文件。私有配置里的 `token_env` 只指定变量名。轮换命令是 Windows 的 `scripts/rotate-token.ps1 -RestartHttp`，或 macOS/Linux 的 `scripts/rotate-token.sh --restart-http`。不要向对话回传 Token 值。

## GPT 快速聊天或远程客户端

先选择数据机器：MCP 地址指向哪台机器，`root` 就配置为那台机器的目录。每人部署自己的实例，不要让请求参数指定任意 IP。当前只读验证可以用私有隧道；长期共享使用可部署稳定 HTTPS 网关。详细网络与认证边界见 [REMOTE_ACCESS.md](REMOTE_ACCESS.md)。

私有隧道的操作顺序：在 Platform 创建 `tunnel_id` 并关联目标 ChatGPT 工作区；在数据机器安装官方 `tunnel-client`；使其指向本机 `safe-local-files stdio --config <私有配置绝对路径>`；运行 `doctor` 确认连接；在 ChatGPT 开发者模式注册 Tunnel 类型 MCP 连接；确认它发现读取工具；新建 GPT 快速聊天，显式启用该连接，仅调用 `stat_path` 检查 `.`。运行密钥仅保存在数据机器私有位置，不能出现在仓库、插件清单、命令输出或对话中。隧道不自动赋予写权限。

HTTPS 地址的操作顺序：为每台数据机器准备可从 ChatGPT 到达的域名和 HTTPS 网关；MCP 进程保持回环监听，设 `behind_proxy=true`；网关验证 ChatGPT 客户端 mTLS，终端用户使用受支持的 OAuth 2.1 身份提供商；将请求转发至同机 `/mcp`，静态后端 Token 只保存在网关私有配置。先验证未认证请求被拒绝、代理不可访问其他路径，再在 ChatGPT 开发者模式注册 HTTPS MCP 连接。**ChatGPT 不支持自定义静态 API key；不能把现有 Token 填进插件 URL 代替 OAuth。** 没有域名、网关和认证时，不要宣称 HTTPS 路线已可用。

## 验收

1. `codex mcp get safe_local_files` 显示 `enabled: true` 和 `transport: stdio`。
2. 私有配置中的 `root` 指向用户自己的目录；端口不是本机连接的依赖。
3. 新建 Codex 任务，只调用 `stat_path`，参数 `{"path":"."}`。确认返回类型为 `directory`，不列举实际文件。若目标是 GPT 快速聊天，还需在该聊天中观察到成功的 MCP 工具调用。
4. 如果写权限仍关闭，工具列表不得出现 `write_file` 或 `create_directory`。
5. 检查本地 Git 状态和忽略规则，确保私有配置、Token、日志、二进制及授权目录内容均未加入版本控制。

遇到问题时先看 `codex mcp get safe_local_files`、根目录是否存在、程序能否启动，再看私有目录下的审计日志。不要为排障把 Token 或用户文件内容贴到公开 Issue。
