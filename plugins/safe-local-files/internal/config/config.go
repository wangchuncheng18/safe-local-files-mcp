package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Root                   string           `json:"root"`
	Listen                 string           `json:"listen"`
	Port                   int              `json:"port"`
	TokenEnv               string           `json:"token_env"`
	AuthMode               string           `json:"auth_mode"`
	CloudflareTeamDomain   string           `json:"cloudflare_team_domain"`
	CloudflareAudience     string           `json:"cloudflare_audience"`
	AuditLog               string           `json:"audit_log"`
	AllowRemote            bool             `json:"allow_remote"`
	BehindProxy            bool             `json:"behind_proxy"`
	AllowRemoteWrite       bool             `json:"allow_remote_write"`
	TLSCertFile            string           `json:"tls_cert_file"`
	TLSKeyFile             string           `json:"tls_key_file"`
	FollowSymlinks         bool             `json:"follow_symlinks"`
	DenyGlobs              []string         `json:"deny_globs"`
	AllowExtensions        []string         `json:"allow_extensions"`
	MaxFileBytes           int64            `json:"max_file_bytes"`
	MaxResponseBytes       int              `json:"max_response_bytes"`
	MaxSearchFiles         int              `json:"max_search_files"`
	MaxSearchResults       int              `json:"max_search_results"`
	SearchTimeoutMS        int              `json:"search_timeout_ms"`
	MaxConcurrency         int              `json:"max_concurrency"`
	RequestsPerMinute      int              `json:"requests_per_minute"`
	SensitiveContentAction string           `json:"sensitive_content_action"`
	MaxWriteBytes          int64            `json:"max_write_bytes"`
	WritePermissions       WritePermissions `json:"write_permissions"`

	Token      string        `json:"-"`
	ConfigDir  string        `json:"-"`
	SearchTime time.Duration `json:"-"`
}

type WritePermissions struct {
	Enabled           bool `json:"enabled"`
	CreateFiles       bool `json:"create_files"`
	OverwriteFiles    bool `json:"overwrite_files"`
	CreateDirectories bool `json:"create_directories"`
}

func Defaults() Config {
	return Config{
		Listen:                 "127.0.0.1",
		Port:                   47381,
		TokenEnv:               "SAFE_LOCAL_FILES_TOKEN",
		AuthMode:               "bearer_token",
		AuditLog:               "./data/audit.jsonl",
		DenyGlobs:              []string{".git", ".git/**", ".ssh", ".ssh/**", ".aws", ".aws/**", ".azure", ".azure/**", ".kube", ".kube/**", "node_modules", "node_modules/**", ".env", ".env.*", "*.pem", "*.key", "*.pfx", "*.p12", "*credential*", "*secret*", "id_rsa*", "id_ed25519*"},
		AllowExtensions:        []string{".txt", ".md", ".markdown", ".json", ".jsonl", ".csv", ".tsv", ".yaml", ".yml", ".toml", ".ini", ".conf", ".log", ".xml", ".html", ".htm", ".css", ".js", ".jsx", ".ts", ".tsx", ".py", ".go", ".rs", ".java", ".c", ".h", ".cpp", ".hpp", ".cs", ".sql", ".sh", ".ps1"},
		MaxFileBytes:           1 << 20,
		MaxResponseBytes:       256 << 10,
		MaxSearchFiles:         10000,
		MaxSearchResults:       100,
		SearchTimeoutMS:        2000,
		MaxConcurrency:         4,
		RequestsPerMinute:      60,
		SensitiveContentAction: "deny",
		MaxWriteBytes:          1 << 20,
	}
}

func Load(path string) (Config, error) {
	return load(path, true)
}

// LoadForStdio loads the filesystem policy without requiring an HTTP bearer
// token. Stdio is bound to client-owned process pipes, not a network listener.
func LoadForStdio(path string) (Config, error) {
	return load(path, false)
}

func load(path string, requireToken bool) (Config, error) {
	cfg := Defaults()
	abs, err := filepath.Abs(path)
	if err != nil {
		return Config{}, err
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	cfg.ConfigDir = filepath.Dir(abs)
	applyEnv(&cfg)
	if err := cfg.normalize(requireToken); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("SLFM_ROOT"); v != "" {
		cfg.Root = v
	}
	if v := os.Getenv("SLFM_LISTEN"); v != "" {
		cfg.Listen = v
	}
	if v := os.Getenv("SLFM_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Port = n
		}
	}
	if v := os.Getenv("SLFM_TOKEN_ENV"); v != "" {
		cfg.TokenEnv = v
	}
	if v := os.Getenv("SLFM_AUDIT_LOG"); v != "" {
		cfg.AuditLog = v
	}
}

func (c *Config) normalize(requireToken bool) error {
	if c.Root == "" {
		return errors.New("root is required")
	}
	if !filepath.IsAbs(c.Root) {
		c.Root = filepath.Join(c.ConfigDir, c.Root)
	}
	root, err := filepath.Abs(c.Root)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("root is unavailable: %w", err)
	}
	if !info.IsDir() {
		return errors.New("root must be a directory")
	}
	c.Root = filepath.Clean(root)

	if c.Listen == "" {
		c.Listen = "127.0.0.1"
	}
	ip := net.ParseIP(c.Listen)
	if ip == nil {
		return errors.New("listen must be an IP address")
	}
	if requireToken && !c.AllowRemote && !ip.IsLoopback() {
		return errors.New("non-loopback listen address requires allow_remote=true")
	}
	if requireToken && !ip.IsLoopback() {
		if c.TLSCertFile == "" || c.TLSKeyFile == "" {
			return errors.New("non-loopback listen address requires tls_cert_file and tls_key_file")
		}
	}
	if requireToken && c.BehindProxy && !ip.IsLoopback() {
		return errors.New("behind_proxy requires a loopback listen address")
	}
	if requireToken && c.HasWriteTools() && (!ip.IsLoopback() || c.BehindProxy) && !c.AllowRemoteWrite {
		return errors.New("remote or proxied write access requires allow_remote_write=true")
	}
	if (c.TLSCertFile == "") != (c.TLSKeyFile == "") {
		return errors.New("tls_cert_file and tls_key_file must be configured together")
	}
	if c.TLSCertFile != "" {
		if !filepath.IsAbs(c.TLSCertFile) {
			c.TLSCertFile = filepath.Join(c.ConfigDir, c.TLSCertFile)
		}
		if !filepath.IsAbs(c.TLSKeyFile) {
			c.TLSKeyFile = filepath.Join(c.ConfigDir, c.TLSKeyFile)
		}
		if _, err := os.Stat(c.TLSCertFile); err != nil {
			return fmt.Errorf("TLS certificate unavailable: %w", err)
		}
		if _, err := os.Stat(c.TLSKeyFile); err != nil {
			return fmt.Errorf("TLS key unavailable: %w", err)
		}
	}
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if c.TokenEnv == "" {
		return errors.New("token_env is required")
	}
	c.Token = os.Getenv(c.TokenEnv)
	if c.AuthMode != "bearer_token" && c.AuthMode != "cloudflare_access" {
		return errors.New("auth_mode must be bearer_token or cloudflare_access")
	}
	if requireToken && c.AuthMode == "bearer_token" && len(c.Token) < 32 {
		return fmt.Errorf("environment variable %s must contain a token of at least 32 characters", c.TokenEnv)
	}
	if requireToken && c.AuthMode == "cloudflare_access" {
		if !c.BehindProxy || !ip.IsLoopback() {
			return errors.New("cloudflare_access requires behind_proxy=true and a loopback listener")
		}
		if !validCloudflareTeamDomain(c.CloudflareTeamDomain) {
			return errors.New("cloudflare_team_domain must be https://<team>.cloudflareaccess.com")
		}
		if len(c.CloudflareAudience) < 16 || len(c.CloudflareAudience) > 256 || strings.ContainsAny(c.CloudflareAudience, " \t\r\n") {
			return errors.New("cloudflare_audience must be a nonempty Access application AUD tag")
		}
	}
	if c.AuditLog == "" {
		return errors.New("audit_log is required")
	}
	if !filepath.IsAbs(c.AuditLog) {
		c.AuditLog = filepath.Join(c.ConfigDir, c.AuditLog)
	}
	c.AuditLog = filepath.Clean(c.AuditLog)
	if c.MaxFileBytes < 1 || c.MaxFileBytes > 64<<20 {
		return errors.New("max_file_bytes must be between 1 and 67108864")
	}
	if c.MaxResponseBytes < 1024 || c.MaxResponseBytes > 4<<20 {
		return errors.New("max_response_bytes must be between 1024 and 4194304")
	}
	if c.MaxWriteBytes < 1 || c.MaxWriteBytes > 64<<20 {
		return errors.New("max_write_bytes must be between 1 and 67108864")
	}
	if !c.WritePermissions.Enabled {
		c.WritePermissions.CreateFiles = false
		c.WritePermissions.OverwriteFiles = false
		c.WritePermissions.CreateDirectories = false
	}
	if c.MaxSearchFiles < 1 || c.MaxSearchResults < 1 || c.MaxConcurrency < 1 || c.RequestsPerMinute < 1 {
		return errors.New("search, concurrency, and rate limits must be positive")
	}
	if c.SearchTimeoutMS < 100 || c.SearchTimeoutMS > 60000 {
		return errors.New("search_timeout_ms must be between 100 and 60000")
	}
	c.SearchTime = time.Duration(c.SearchTimeoutMS) * time.Millisecond
	c.SensitiveContentAction = strings.ToLower(c.SensitiveContentAction)
	if c.SensitiveContentAction != "deny" && c.SensitiveContentAction != "redact" {
		return errors.New("sensitive_content_action must be deny or redact")
	}
	for i, ext := range c.AllowExtensions {
		ext = strings.ToLower(strings.TrimSpace(ext))
		if ext != "" && !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		c.AllowExtensions[i] = ext
	}
	return nil
}

func validCloudflareTeamDomain(value string) bool {
	if !strings.HasPrefix(value, "https://") {
		return false
	}
	host := strings.TrimPrefix(value, "https://")
	if !strings.HasSuffix(host, ".cloudflareaccess.com") || strings.ContainsAny(host, "/?#@ :\\") {
		return false
	}
	team := strings.TrimSuffix(host, ".cloudflareaccess.com")
	if team == "" || len(team) > 63 || strings.HasPrefix(team, "-") || strings.HasSuffix(team, "-") {
		return false
	}
	for _, ch := range team {
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
			return false
		}
	}
	return true
}

func (c Config) Address() string {
	return net.JoinHostPort(c.Listen, strconv.Itoa(c.Port))
}

func (c Config) HasWriteTools() bool {
	w := c.WritePermissions
	return w.Enabled && (w.CreateFiles || w.OverwriteFiles || w.CreateDirectories)
}
