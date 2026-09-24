# Threat Model

## Assets

- Files below the configured root.
- The bearer token.
- Runtime configuration and audit records.
- Workstation availability and I/O performance.

## Trust boundaries

- MCP client to a local stdio process or optional authenticated HTTP server.
- Requested relative paths to the configured filesystem root.
- File content to the language model. File content is untrusted data.
- Local user account to other local users and processes.

## Addressed threats

| Threat | Control |
| --- | --- |
| Path traversal | Canonical relative-path validation and root containment checks |
| Symlink escape | Symlink rejection by default; resolved containment check when enabled |
| Unauthenticated access | Constant-time bearer-token comparison |
| Accidental LAN exposure | Loopback-only default; explicit `allow_remote` gate |
| Accidental remote write through a loopback reverse proxy | Explicit `behind_proxy` mode requires `allow_remote_write` before write tools can be enabled |
| Secret disclosure | Denied filenames, extension allowlist, content detectors, search redaction |
| Resource exhaustion | Rate, concurrency, body, file, response, scan, result, and time limits |
| Hidden background load | No indexing, no watcher, work only during calls |
| Audit leakage | No tokens, queries, or file contents in logs; queries are hashed |
| Prompt injection from files | Plugin skill declares file content untrusted and non-authoritative |
| Unauthorized modification | No write tools by default; independent create, overwrite, and directory flags; confined filesystem operations and size/content policy |

## Residual risks

- A process running as the same operating-system user may read environment variables or runtime files permitted to that user.
- Pattern-based secret detection cannot identify every sensitive datum. Narrow roots and deny rules remain the primary controls.
- Files that are legitimately readable can contain personal or confidential data. The operator must scope the root and allowlist appropriately.
- Non-loopback mode requires TLS and a bearer token, but a compromised client or leaked token can still invoke all enabled operations. Use a private VPN and a narrow root.
- A reverse proxy makes a loopback listener remotely reachable. The operator must set `behind_proxy`, enforce TLS and user authentication at the gateway, and keep the backend bearer token private. The application cannot detect an unannounced proxy automatically.
- Audit logs show relative filenames and access timing. Protect and rotate them according to local policy.

## Explicit non-goals

- Multi-user authorization.
- Serving raw MCP directly on the public internet, or providing OAuth/user identity within this single-instance server.
- Deleting, moving, or transforming existing files beyond explicit text overwrite.
- Full-text indexing of large repositories.
- Malware scanning or data-loss-prevention classification.
