# 无域名的私有 GPT 快速聊天连接

这条路线让 GPT 快速聊天读取**自己机器上**授权的一个目录，无需购买域名、配置 Cloudflare 或开放入站端口。链路是 `GPT 快速聊天 → OpenAI Secure MCP Tunnel → 本机 tunnel-client → 本机 stdio MCP → 私有 root`。Tunnel 是私有连接，用于个人或工作区内测试；它不能代替公开发布插件所要求的稳定公网 HTTPS MCP 地址。[OpenAI Secure MCP Tunnel](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels)

每个人在自己的机器安装自己的 Safe Local Files，并在自己的 OpenAI Platform 组织中建立 `tunnel_id` 与运行密钥。仓库里**不能**预置作者的 `tunnel_id`、API Key 或目录路径。每个人的 ChatGPT 开发者模式 MCP 连接也需要单独注册。

## 交给本地 Codex 的任务

> 请阅读本仓库 README.md、INSTALL_FOR_AGENTS.md 和 SECURE_TUNNEL.md，为这台 [Windows/macOS/Linux] 机器的 [绝对路径] 建立只读 Safe Local Files。先用 stdio MCP 的 `stat_path({"path":"."})` 验证它是目录，不读取或列举文件。然后从 OpenAI 官方发布页安装并校验 tunnel-client；让我在浏览器自行登录 Platform，并在需要时创建属于我账号的 Tunnel 和运行 API Key。密钥只保存到本机私有文件，不打印、不贴进聊天、不提交 Git。用 `file:` 密钥引用与本机 stdio 命令连接 Tunnel，运行 doctor 和 managed runtime status；最后在我的 ChatGPT 开发者模式注册 Tunnel 类型 MCP 连接，在新的 GPT 快速聊天里只调用 `stat_path(".")` 验证。没有实际工具调用成功前，不要声称已接通；始终保持写入关闭。

## 前置条件

1. 安装本仓库，设置私有 `config.json` 的 `root` 为自己机器的目录，保持 `write_permissions.enabled=false`。本机 Codex 的 `codex mcp get safe_local_files` 应显示 `stdio`，并可用 `stat_path(".")` 检查根目录。见 [README](README.md#五分钟安装)。
2. 能登录 [OpenAI Platform 的 Tunnel 设置](https://platform.openai.com/settings/organization/tunnels)；创建或管理 Tunnel 需要对应组织的 **Tunnels Read + Manage** 权限，运行客户端需要 **Read + Use**。把 Tunnel 关联到自己的 ChatGPT 工作区。ChatGPT 还需要开发者模式权限。这些权限是否对某个具体账号开放，必须以账号页面为准。[官方权限说明](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels)
3. 从 OpenAI 官方 [tunnel-client 最新 Release](https://github.com/openai/tunnel-client/releases/latest) 下载对应 Windows/macOS/Linux 架构的包并用同版 `SHA256SUMS.txt` 核验。运行 `tunnel-client help quickstart`，以安装的版本帮助为准。不要从第三方下载可执行文件。

## 私有凭据与启动

在 Platform Tunnel 设置中创建或选取自己的 Tunnel，记录它的 `tunnel_id`。在 Runtime API keys 创建供 tunnel-client 使用的运行密钥；它与创建 Tunnel 的管理权限是分开的。将密钥保存在**授权 root 和 Git 仓库之外**的当前用户私有文件中，只给当前用户读取。不要把密钥值放在命令行参数、日志、仓库或对话里，也不要使用组织 Admin API Key 作为长驻客户端的运行密钥。

Windows 建议保存到 `%LOCALAPPDATA%\SafeLocalFiles\openai-tunnel-api-key`；macOS/Linux 保存到 `${XDG_CONFIG_HOME:-$HOME/.config}/safe-local-files/openai-tunnel-api-key` 并设置 `chmod 600`。`tunnel-client` 支持引用 `file:<密钥文件绝对路径>`，这样配置只保存文件路径。Windows 上可在项目根目录执行：

```powershell
$client = "$env:LOCALAPPDATA\SafeLocalFiles\tools\tunnel-client.exe"
$mcp = (Resolve-Path .\plugins\safe-local-files\bin\safe-local-files.exe).Path
$cfg = "$env:LOCALAPPDATA\SafeLocalFiles\config.json"
$key = "$env:LOCALAPPDATA\SafeLocalFiles\openai-tunnel-api-key"
$mcpCommand = '"{0}" stdio --config "{1}"' -f $mcp, $cfg
& $client runtimes connect --alias wccwinpc --tunnel-id '<自己的 tunnel_id>' --runtime-api-key "file:$key" --mcp-command $mcpCommand
& $client runtimes status wccwinpc --json
```

macOS/Linux 可在项目根目录执行：

```sh
MCP_BIN="$(pwd)/plugins/safe-local-files/bin/safe-local-files"
CONFIG="${XDG_CONFIG_HOME:-$HOME/.config}/safe-local-files/config.json"
KEY_FILE="${XDG_CONFIG_HOME:-$HOME/.config}/safe-local-files/openai-tunnel-api-key"
tunnel-client runtimes connect --alias wccwinpc --tunnel-id '<自己的 tunnel_id>' \
  --runtime-api-key "file:$KEY_FILE" \
  --mcp-command "\"$MCP_BIN\" stdio --config \"$CONFIG\""
tunnel-client runtimes status wccwinpc --json
```

只有 status 显示进程运行且健康、就绪时才继续。需要检查连接问题时，先运行 `tunnel-client doctor --profile <创建的 profile> --explain`；停止本机连接用 `tunnel-client runtimes stop wccwinpc`。停止不删除远端 Tunnel。不同版本的命令可能变化，以已安装客户端的 `help quickstart` 和 `runtimes connect --help` 为准。

## 接入快速聊天并验收

在 ChatGPT 开发者模式中打开 [Plugins](https://chatgpt.com/plugins)，添加 MCP 连接，**Connection 选择 Tunnel**，选取或填写自己的 `tunnel_id`。必须在本机 tunnel-client 正常运行时创建连接，确认工具发现包含 `stat_path`、`read_file` 等只读工具且没有 `write_file`、`create_directory`。新建 GPT 快速聊天，启用该连接，只让它调用 `stat_path({"path":"."})`。看到真实工具调用并返回 `directory` 才算成功。[官方 ChatGPT 接入步骤](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels)

这条链路只为该用户自己的账号或被关联的工作区提供私有访问。朋友若想让自己的快速聊天读自己的电脑，必须在朋友的机器上重复安装，使用朋友自己的授权目录、Platform Tunnel、运行密钥和 ChatGPT 连接。克隆仓库本身不会让任何人的文件自动公开。
