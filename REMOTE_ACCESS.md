# 跨机器访问设计

本机使用时选 `stdio`。如果客户端和数据不在同一台电脑，`127.0.0.1` 在客户端看来指的是客户端自己，不能访问数据机器。跨机器必须建立一条从客户端到数据机器的网络路径。

推荐顺序：在两台机器之间建立私有 VPN（如 WireGuard/Tailscale 等），只对 VPN 网卡地址提供 HTTPS MCP 服务，再在客户端配置该地址及独立的 Bearer Token。只给可信设备加入 VPN，限制主机防火墙入站来源。HTTP 服务不应绑定 `0.0.0.0` 后直接做公网端口映射。

服务器私有配置中的关键字段：

```json
{
  "listen": "<数据机器的 VPN 网卡 IP>",
  "port": 47381,
  "allow_remote": true,
  "allow_remote_write": false,
  "tls_cert_file": "<证书绝对路径>",
  "tls_key_file": "<私钥绝对路径>"
}
```

`root`、Token、审计日志和其他限制仍在同一份配置中。非回环地址必须显式启用 `allow_remote` 并提供 TLS 证书与私钥，服务才会启动。证书需对客户端访问时使用的主机名/IP 有效，私钥应放在授权根目录**之外**并只允许服务账号读取。客户端应验证证书，不要关闭 TLS 校验。HTTP Token 至少 32 字符；部署时通过环境变量注入，不写入仓库。

远程写入还有第二道开关：仅当 `allow_remote_write=true` 且 `write_permissions` 中相应操作也打开时，非回环监听才允许暴露写工具。可先保持远程只读。即使使用 VPN，也要把授权根目录缩小到专用目录，并定期查看审计日志和轮换 Token。

在**客户端机器**上，把通过安全渠道得到的 Token 存为客户端自己的环境变量，例如 `SAFE_LOCAL_FILES_REMOTE_TOKEN`，再注册远端地址：

```sh
codex mcp add safe_local_files_remote --url https://<VPN-IP-or-private-hostname>:47381/mcp --bearer-token-env-var SAFE_LOCAL_FILES_REMOTE_TOKEN
```

客户端须信任服务端 TLS 证书，并新建 Codex 对话。不要把 Token 直接放在 URL、命令行参数、插件清单或 Git 仓库中。服务器和客户端可以是 Windows 或 macOS 的任意组合；目录路径始终以**服务器机器**的 `root` 为准，客户端只提供相对路径。

如果只需偶尔从另一台电脑读取，也可使用已有的受信任 HTTPS 反向代理或私有隧道，将本机回环 HTTP 端点安全转发。代理必须负责 TLS、访问控制和来源限制；不要把裸 HTTP 或 Token 传输暴露在公网。不同隧道产品的命令会变化，本项目不自动修改路由器或防火墙，也不自动发布公共 URL。

公网端口映射会扩大攻击面：密码猜测、Token 泄露、配置错误、扫描和拒绝服务风险都由数据机器承担。本项目尚未实现 OAuth、账户分级、远程设备撤销或公网级防护，因此**不推荐直接公网部署**。需要长期多人访问时，应另用成熟的 VPN、身份提供商和网关管理。
