# Cloudflare Tunnel + Access 部署

这条路线用于 GPT 快速聊天访问自己电脑或远程服务器上的一个授权目录。每台数据机器各自安装 Safe Local Files，设置自己的 `root`、Cloudflare 域名、Access 应用和 Tunnel。MCP 地址选择数据机器，工具参数只有该机器 `root` 内的相对路径。默认只读；此文不要求开放路由器端口。

朋友从零开始可先读 [QUICK_CHAT_SETUP.md](QUICK_CHAT_SETUP.md)。统一命名建议：Tunnel 用 `safe-local-files`，主机名用 `files.<自己的域名>`；尖括号中的域名必须由部署者控制。

## 前置条件

1. 一个 Cloudflare 账号，以及已接入该账号的域名。固定的 `https://files.example.com/mcp` 需要域名；随机 `trycloudflare.com` Quick Tunnel 仅供临时测试，不能作为长期插件地址。[Cloudflare Tunnel 要求](https://developers.cloudflare.com/tunnel/get-started/)
2. 已安装并运行 Safe Local Files；Windows、macOS 和 Linux 参见 [README.md](README.md)。当前 Windows 上 `cloudflared` 可通过 `winget install --id Cloudflare.cloudflared --exact` 安装；其他系统用 [Cloudflare 官方下载](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/downloads/)。
3. Cloudflare Zero Trust 团队域名（形如 `https://myteam.cloudflareaccess.com`），用于登录的身份方式，以及仅允许本人访问的 Access 策略。

## 先配置源站，保持失败即关闭

编辑数据机器的私有 `config.json`。保留原有 `root`、审计和资源限制，设置以下字段；`cloudflare_audience` 是稍后创建的 Access 应用的 **Application Audience (AUD) Tag**，不是域名或 Tunnel ID。创建 Access 应用前先保持 MCP 服务停止。

```json
{
  "listen": "127.0.0.1",
  "port": 47381,
  "allow_remote": false,
  "behind_proxy": true,
  "auth_mode": "cloudflare_access",
  "cloudflare_team_domain": "https://myteam.cloudflareaccess.com",
  "cloudflare_audience": "<Access 应用的 AUD tag>",
  "allow_remote_write": false,
  "write_permissions": {
    "enabled": false,
    "create_files": false,
    "overwrite_files": false,
    "create_directories": false
  }
}
```

Cloudflare 模式逐请求验证 `Cf-Access-Jwt-Assertion` 的 RS256 签名、`iss`、`aud`、有效期和生效时间；签名公钥仅从配置的 Cloudflare 团队域名取得。缺少 JWT 的本机 HTTP 请求也会被拒绝。`stdio` 模式仍可供本机 Codex 使用，HTTP 静态 Token 不会被当作 Cloudflare 登录凭据。[Cloudflare JWT 验证要求](https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/authorization-cookie/validating-json/)

在开始 Tunnel 前运行 `safe-local-files validate --config <私有配置绝对路径> --require-cloudflare-read-only`。这项检查要求回环监听、Cloudflare Access 鉴权以及所有远程和写入开关关闭；两种启动脚本也会自动执行它。

## 在 Cloudflare 建立认证与固定地址

1. 在 Zero Trust 配置身份提供商或 One-time PIN。创建仅允许本人身份的 Access 策略。
2. 在 Zero Trust → Access controls → Applications 创建保护**整个** `files.example.com` 主机名的 **MCP server application**（或官方支持的自托管应用），不要只保护 `/mcp` 而漏出其他路径。启用 **Managed OAuth**，记录团队域名及 Application Audience (AUD) Tag，并将 AUD tag 写入本机私有配置。Managed OAuth 让 MCP 客户端完成标准 OAuth 登录；源站仍须验证 Access JWT。[Cloudflare Managed OAuth](https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/managed-oauth/)
3. 在 Cloudflare Networking → Tunnels 创建 Named Tunnel。添加 `files.example.com` 的 Published application route，Service URL 指向数据机器的 `http://127.0.0.1:47381`。不要把未经 Access 保护的其他主机名指向同一服务。[Tunnel 路由步骤](https://developers.cloudflare.com/tunnel/get-started/)
4. 将 Tunnel 运行 Token **只保存于该机器的私有文件**，不要提交仓库、放进 URL 或贴进对话。Windows 建议放在 `%LOCALAPPDATA%\SafeLocalFiles\cloudflare-tunnel-token` 并限制为当前用户可读；macOS/Linux 可放在 `${XDG_CONFIG_HOME:-$HOME/.config}/safe-local-files/cloudflare-tunnel-token` 并设 `chmod 600`。Cloudflare 的账号级证书与 Tunnel Token 权限不同；此处只需运行单个 Tunnel 的 Token。

## 启动与检查

Windows，在仓库根目录：

```powershell
.\plugins\safe-local-files\scripts\start-cloudflared.ps1
# 停止时：
.\plugins\safe-local-files\scripts\stop-cloudflared.ps1
.\plugins\safe-local-files\scripts\stop.ps1
```

macOS/Linux，安装 `cloudflared` 后在仓库根目录：

```sh
./plugins/safe-local-files/scripts/start-cloudflared.sh
# 前台按 Ctrl+C 停止 Tunnel；另运行 ./plugins/safe-local-files/scripts/stop.sh 停止 MCP HTTP 服务
```

先用本机无凭据请求 `http://127.0.0.1:47381/mcp`，确认返回 `401`。然后在 Cloudflare 仪表板确认 Tunnel 健康，并通过未登录浏览器访问公开地址，确认它要求 Access 登录，不能直接返回目录或 MCP 工具。完成登录后，在 ChatGPT 开发者模式注册 `https://files.example.com/mcp`，确认连接发现 `stat_path` 等读取工具；在新的 GPT 快速聊天中选择该 MCP 连接，只调用 `stat_path({"path":"."})` 验证目录类型。不要用列目录或读文件作为首次验证。

插件包要在 GPT 快速聊天中自动携带该连接，还需把 ChatGPT 创建的 `plugin_asdk_app...` 技术 ID 加入插件 `.app.json` 并重新安装。这个 ID 是每位使用者各自注册连接后得到的，不能把作者的 ID 预置给朋友。[OpenAI 插件映射步骤](https://developers.openai.com/plugins/build/plugins)

本项目尚未在真实 Cloudflare 账号和 GPT 快速聊天上完成端到端验证；只有观察到成功的工具调用，才能称为接通。不要在此之前公开授权目录或开启写入。
