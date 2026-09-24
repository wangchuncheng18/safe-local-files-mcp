# Security Policy

## Supported versions

Security fixes are applied to the latest release and the default branch.

## Reporting a vulnerability

Do not open a public issue for a vulnerability that could expose local files, tokens, audit records, or path-policy bypasses. Use GitHub's private vulnerability reporting feature for this repository.

Include the affected version, operating system, configuration with secrets removed, reproduction steps, and impact. Never attach real credentials or sensitive files.

## Deployment requirements

- Keep `listen` on a loopback address for normal use. Non-loopback binding requires explicit opt-in and TLS certificate/key files; restrict it to a VPN address and trusted clients.
- Use a unique token of at least 32 characters and rotate it after suspected exposure.
- Put only the narrowest necessary directory under `root`.
- Keep `follow_symlinks` disabled unless the target layout has been reviewed.
- Review `deny_globs` and `allow_extensions` for the data set.
- Protect the runtime config, token store, and audit log with operating-system access controls.
- Do not run the service as administrator/root.

Write tools are absent by default. The owner may separately enable text-file creation, text-file overwrite, and directory creation in private configuration. The server has no rename, move, delete, command execution, upload, or arbitrary URL tools. Keep write permissions disabled for sensitive or production directories.
