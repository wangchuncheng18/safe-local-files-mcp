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
	Root                   string   `json:"root"`
	Listen                 string   `json:"listen"`
	Port                   int      `json:"port"`
	TokenEnv               string   `json:"token_env"`
	AuditLog               string   `json:"audit_log"`
	AllowRemote            bool     `json:"allow_remote"`
	FollowSymlinks         bool     `json:"follow_symlinks"`
	DenyGlobs              []string `json:"deny_globs"`
	AllowExtensions        []string `json:"allow_extensions"`
	MaxFileBytes           int64    `json:"max_file_bytes"`
	MaxResponseBytes       int      `json:"max_response_bytes"`
	MaxSearchFiles         int      `json:"max_search_files"`
	MaxSearchResults       int      `json:"max_search_results"`
	SearchTimeoutMS        int      `json:"search_timeout_ms"`
	MaxConcurrency         int      `json:"max_concurrency"`
	RequestsPerMinute      int      `json:"requests_per_minute"`
	SensitiveContentAction string   `json:"sensitive_content_action"`

	Token      string        `json:"-"`
	ConfigDir  string        `json:"-"`
	SearchTime time.Duration `json:"-"`
}

func Defaults() Config {
	return Config{
		Listen:                 "127.0.0.1",
		Port:                   8765,
		TokenEnv:               "SAFE_LOCAL_FILES_TOKEN",
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
	}
}

func Load(path string) (Config, error) {
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
	if err := cfg.normalize(); err != nil {
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

func (c *Config) normalize() error {
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
	if !c.AllowRemote && !ip.IsLoopback() {
		return errors.New("non-loopback listen address requires allow_remote=true")
	}
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if c.TokenEnv == "" {
		return errors.New("token_env is required")
	}
	c.Token = os.Getenv(c.TokenEnv)
	if len(c.Token) < 32 {
		return fmt.Errorf("environment variable %s must contain a token of at least 32 characters", c.TokenEnv)
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

func (c Config) Address() string {
	return net.JoinHostPort(c.Listen, strconv.Itoa(c.Port))
}
