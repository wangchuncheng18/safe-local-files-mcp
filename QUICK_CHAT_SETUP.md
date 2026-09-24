# 让 GPT 快速聊天只读访问自己的文件夹

本指南适用于 Windows、macOS 和 Linux。每个人在**存放文件的那台机器**部署自己的 Safe Local Files，指定自己的授权目录，并在自己的 ChatGPT 账号里注册自己的 MCP 地址。GitHub 仓库只提供代码；它不包含作者的目录、Cloudflare 凭据或任何可复用的个人连接 ID。先实现只读；不要开启写权限。

## 连接链路

`GPT 快速聊天 → 你的 HTTPS 主机名 → Cloudflare Access 登录 → Cloudflare Tunnel → 本机 127.0.0.1:47381/mcp → 你配置的 root`

建议把 Tunnel 命名为 `safe-local-files`，把主机名设为 `files.<你自己的域名>`。这只是命名约定：`<你自己的域名>` 必须由你控制并已接入 Cloudflare，不能随便填写一个尚未注册或不属于你的域名。Cloudflare 的随机 Quick Tunnel 地址只适合临时调试，不能充当长期连接地址。[Cloudflare Tunnel 前置条件](https://developers.cloudflare.com/tunnel/get-started/)

## 交给朋友的 Codex 的任务

朋友可以把下面这段话粘贴给**自己机器上的 Codex**，并把方括号中的值换成自己的。执行过程中由本人在 Cloudflare 和 ChatGPT 登录；不要把密码、Tunnel Token、Access JWT 或文件内容贴给助手。

> 请按 https://github.com/wangchuncheng18/safe-local-files-mcp 的 README.md、INSTALL_FOR_AGENTS.md、QUICK_CHAT_SETUP.md 和 CLOUDFLARE.md，在这台 [Windows/macOS/Linux] 机器部署只读 Safe Local Files。我的唯一授权目录是 [绝对路径]；我控制并已接入 Cloudflare 的域名是 [域名]。请安装、配置、验证本机 MCP；把 Tunnel 命名为 safe-local-files，主机名使用 files.[域名]；配置只允许我本人登录的 Cloudflare Access MCP 应用并开启 Managed OAuth；把 Tunnel 运行 Token 仅保存在本机私有文件。然后在我的 ChatGPT 开发者模式注册 https://files.[域名]/mcp，在新的 GPT 快速聊天中只用 stat_path 检查 `.`。不要读取或列出我的文件作为安装测试，不要开启任何写权限。每一步给出实际测试结果；若缺少域名、账号登录或连接 ID，请说明具体待办，不要声称已接通。

## 安装到数据机器

安装 Git、Go 1.25+ 和 Codex CLI，然后按 [README.md](README.md#五分钟安装) 克隆并运行对应系统的 `install.ps1` 或 `install.sh`。安装器先建立本机 `stdio` MCP；这让本机 Codex 能使用工具，**不会自动把工具送到 GPT 快速聊天**。将私有 `config.json` 的 `root` 改为这台机器上已存在的授权目录。Windows 私有配置在 `%LOCALAPPDATA%\SafeLocalFiles\config.json`；macOS/Linux 在 `${XDG_CONFIG_HOME:-$HOME/.config}/safe-local-files/config.json`。保持全部 `write_permissions` 为 `false`。

先运行 `codex mcp get safe_local_files`，再新建 Codex 任务，只测试 `stat_path` 的 `{"path":"."}`。应得到目录元数据；工具列表中不应有 `write_file` 或 `create_directory`。不要从作者的 `E:\工作` 复制路径到朋友机器。

## 建立自己的 Cloudflare 地址

1. 将自己控制的域名接入 Cloudflare，并在 Zero Trust 建立仅允许自己身份登录的 Access 策略。若还没有域名，停在这里；仅“起一个域名名字”不会使地址生效。
2. 在 Zero Trust 创建覆盖**整个** `files.<你的域名>` 主机名的 MCP server application，启用 **Managed OAuth**，记下团队域名（例如 `https://myteam.cloudflareaccess.com`）和该应用的 **Application Audience (AUD) Tag**。Access 登录策略必须只放行本人。[Cloudflare Managed OAuth](https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/managed-oauth/)
3. 按 [CLOUDFLARE.md](CLOUDFLARE.md) 修改这台机器的私有配置：`auth_mode=cloudflare_access`、`behind_proxy=true`、`listen=127.0.0.1`，填入团队域名和 AUD tag。保留 `allow_remote=false`、`allow_remote_write=false`、全部写权限关闭。运行 `safe-local-files validate --config <私有配置> --require-cloudflare-read-only`；失败就不要启动 Tunnel。
4. 在 Cloudflare Networking → Tunnels 创建名为 `safe-local-files` 的 remotely managed Tunnel。给它添加 `files.<你的域名>` 的 Published application route，Service URL 指向**同一台机器**的 `http://127.0.0.1:47381`。如果改过端口，此处也要同步。保存 Tunnel 运行 Token 到本机私有文件，按 [CLOUDFLARE.md](CLOUDFLARE.md) 启动。不要把 Token 放进 GitHub、URL 或聊天记录。

启动脚本会在连接 Cloudflare 前检查配置确实处于只读 Cloudflare Access 模式。Windows 运行 `plugins/safe-local-files/scripts/start-cloudflared.ps1`；macOS/Linux 运行 `plugins/safe-local-files/scripts/start-cloudflared.sh`。电脑关机或 Tunnel 进程停止时，GPT 无法访问这台机器。

## 接入 ChatGPT 快速聊天并验收

1. 在本机无凭据 POST `http://127.0.0.1:47381/mcp`，必须得到 HTTP `401`。未登录浏览器访问 `https://files.<你的域名>/mcp`，必须先进入 Cloudflare Access 登录，不能直接返回 MCP 工具或目录。Cloudflare 控制台应显示 Tunnel 正常。
2. 在 ChatGPT 的 **Settings → Security and login** 开启 Developer mode；到 ChatGPT Plugins 创建 MCP 连接，地址填 `https://files.<你的域名>/mcp`，完成 Cloudflare OAuth 登录。连接创建后应能发现 `stat_path` 等读取工具。[OpenAI 插件接入步骤](https://developers.openai.com/plugins/build/plugins)
3. 新建 GPT 快速聊天，显式启用刚创建的 MCP 连接，发送“只调用 `stat_path` 检查授权根目录 `.`，不要列目录或读文件”。只有在这段聊天中实际看到工具调用并返回目录元数据，才算目标达成。确认工具列表没有写入工具。

ChatGPT 为每个人创建的连接有不同的 `plugin_asdk_app...` 技术 ID。**单独注册并启用 MCP 连接就可以测试快速聊天**。如果还想让个人本地插件自动关联这条连接，需要在本机插件的 `.app.json` 中映射自己的 ID，并让插件清单引用它，重新安装插件；这份个人映射不要提交到公共 GitHub。GitHub 上预置作者的连接 ID 无法让朋友访问朋友自己的机器。[OpenAI 插件打包说明](https://developers.openai.com/plugins/build/plugins)

验收记录只需包含：系统、版本、授权根目录是否存在、`codex mcp get` 状态、Cloudflare Tunnel 状态、未登录请求的拒绝结果、ChatGPT 工具发现结果，以及快速聊天的 `stat_path(".")` 是否成功。不要在验收记录中放 Token、JWT、目录列表或文件内容。
