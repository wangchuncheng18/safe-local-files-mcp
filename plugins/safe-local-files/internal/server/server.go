package server

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wangchuncheng18/safe-local-files-mcp/internal/audit"
	"github.com/wangchuncheng18/safe-local-files-mcp/internal/config"
	"github.com/wangchuncheng18/safe-local-files-mcp/internal/safefs"
)

type Service struct {
	cfg     config.Config
	fs      *safefs.FS
	audit   *audit.Logger
	server  *mcp.Server
	sem     chan struct{}
	limiter *rateLimiter
}

type ListInput struct {
	Path  string `json:"path,omitempty" jsonschema:"relative directory path under the configured root; defaults to ."`
	Depth int    `json:"depth,omitempty" jsonschema:"directory recursion depth from 1 to 5"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum entries from 1 to 1000"`
}

type ListOutput struct {
	Entries []safefs.Entry `json:"entries"`
}

type ReadInput struct {
	Path       string `json:"path" jsonschema:"relative file path under the configured root"`
	OffsetLine int    `json:"offset_line,omitempty" jsonschema:"one-based first line to return"`
	MaxLines   int    `json:"max_lines,omitempty" jsonschema:"maximum lines to return, up to 5000"`
}

type ReadOutput struct {
	File safefs.File `json:"file"`
}

type StatInput struct {
	Path string `json:"path" jsonschema:"relative path under the configured root"`
}

type StatOutput struct {
	Entry safefs.Entry `json:"entry"`
}

type SearchInput struct {
	Query string `json:"query" jsonschema:"literal text or RE2 regular expression to find"`
	Path  string `json:"path,omitempty" jsonschema:"relative directory path under the configured root; defaults to ."`
	Regex bool   `json:"regex,omitempty" jsonschema:"interpret query as an RE2 regular expression"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum result count"`
}

type SearchResult struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"`
	Path  string `json:"path"`
	Line  int    `json:"line,omitempty"`
}

type SearchOutput struct {
	Results      []SearchResult `json:"results"`
	ScannedFiles int            `json:"scanned_files"`
}

type FetchInput struct {
	ID string `json:"id" jsonschema:"document ID returned by search; this is a relative file path"`
}

type FetchOutput struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Text      string `json:"text"`
	Path      string `json:"path"`
	Truncated bool   `json:"truncated"`
}

type WriteInput struct {
	Path    string `json:"path" jsonschema:"relative file path under the configured root"`
	Content string `json:"content" jsonschema:"UTF-8 text to write, subject to policy and size limits"`
	Mode    string `json:"mode" jsonschema:"create or overwrite; must be specified explicitly"`
}

type WriteOutput struct {
	Entry safefs.Entry `json:"entry"`
}

type CreateDirectoryInput struct {
	Path string `json:"path" jsonschema:"relative path of one new directory; parent must exist"`
}

func New(cfg config.Config) (*Service, error) {
	fs, err := safefs.New(cfg)
	if err != nil {
		return nil, err
	}
	auditLog, err := audit.New(cfg.AuditLog)
	if err != nil {
		return nil, err
	}
	s := &Service{
		cfg:     cfg,
		fs:      fs,
		audit:   auditLog,
		sem:     make(chan struct{}, cfg.MaxConcurrency),
		limiter: newRateLimiter(cfg.RequestsPerMinute),
	}
	instructions := "Access to one configured local directory. Paths are relative to the root. Secret-like files and content may be denied or redacted. Use search before fetch when locating documents."
	if cfg.HasWriteTools() {
		instructions += " Write tools are restricted by independent configuration flags. Confirm the intended path and content before calling a write tool."
	} else {
		instructions += " This instance is read-only; do not claim write access."
	}
	s.server = mcp.NewServer(
		&mcp.Implementation{Name: "safe-local-files", Version: "v0.2.0"},
		&mcp.ServerOptions{
			Instructions: instructions,
			Capabilities: &mcp.ServerCapabilities{},
		},
	)
	s.registerTools()
	return s, nil
}

func (s *Service) registerTools() {
	annotations := &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: boolPtr(false), DestructiveHint: boolPtr(false)}

	mcp.AddTool(s.server, &mcp.Tool{
		Name: "search", Title: "Search local files",
		Description: "Search allowed text files below the configured root by filename and content. Results are bounded, secret-like lines are redacted, and denied paths are skipped.",
		Annotations: annotations,
	}, s.search)

	mcp.AddTool(s.server, &mcp.Tool{
		Name: "fetch", Title: "Fetch local document",
		Description: "Fetch an allowed text document by the relative path returned as a search result ID.",
		Annotations: annotations,
	}, s.fetch)

	mcp.AddTool(s.server, &mcp.Tool{
		Name: "list_directory", Title: "List local directory",
		Description: "List bounded metadata for files and directories below the configured root. Denied paths and symlinks are skipped by default.",
		Annotations: annotations,
	}, s.list)

	mcp.AddTool(s.server, &mcp.Tool{
		Name: "read_file", Title: "Read local text file",
		Description: "Read a bounded range of lines from an allowed text file below the configured root. Binary, oversized, denied, and secret-like files are blocked by default.",
		Annotations: annotations,
	}, s.read)

	mcp.AddTool(s.server, &mcp.Tool{
		Name: "stat_path", Title: "Inspect local path metadata",
		Description: "Return metadata for one allowed file or directory below the configured root without reading its contents.",
		Annotations: annotations,
	}, s.stat)

	if s.cfg.HasWriteTools() {
		writeAnnotations := &mcp.ToolAnnotations{ReadOnlyHint: false, IdempotentHint: false, OpenWorldHint: boolPtr(false), DestructiveHint: boolPtr(true)}
		if s.cfg.WritePermissions.CreateFiles || s.cfg.WritePermissions.OverwriteFiles {
			mcp.AddTool(s.server, &mcp.Tool{
				Name: "write_file", Title: "Write local text file",
				Description: "Create or overwrite one allowed UTF-8 text file inside the configured root. Requires an explicitly enabled operation and mode; content is never logged.",
				Annotations: writeAnnotations,
			}, s.write)
		}
		if s.cfg.WritePermissions.CreateDirectories {
			mcp.AddTool(s.server, &mcp.Tool{
				Name: "create_directory", Title: "Create local directory",
				Description: "Create one directory under an existing parent inside the configured root. Requires explicit configuration.",
				Annotations: writeAnnotations,
			}, s.createDirectory)
		}
	}
}

func (s *Service) write(_ context.Context, _ *mcp.CallToolRequest, in WriteInput) (*mcp.CallToolResult, WriteOutput, error) {
	started := time.Now()
	var entry safefs.Entry
	var err error
	modeForAudit := in.Mode
	switch in.Mode {
	case "create":
		entry, err = s.fs.WriteFile(in.Path, in.Content, false)
	case "overwrite":
		entry, err = s.fs.WriteFile(in.Path, in.Content, true)
	default:
		modeForAudit = "invalid"
		err = fmt.Errorf("mode must be create or overwrite")
	}
	bytesWritten := 0
	if err == nil {
		bytesWritten = len(in.Content)
	}
	s.recordWithDetail("write_file", in.Path, 0, bytesWritten, started, err, "mode="+modeForAudit)
	return nil, WriteOutput{Entry: entry}, err
}

func (s *Service) createDirectory(_ context.Context, _ *mcp.CallToolRequest, in CreateDirectoryInput) (*mcp.CallToolResult, WriteOutput, error) {
	started := time.Now()
	entry, err := s.fs.CreateDirectory(in.Path)
	s.record("create_directory", in.Path, 0, 0, started, err)
	return nil, WriteOutput{Entry: entry}, err
}

func (s *Service) list(_ context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, ListOutput, error) {
	started := time.Now()
	path := defaultPath(in.Path)
	entries, err := s.fs.List(path, in.Depth, in.Limit)
	s.record("list_directory", path, len(entries), 0, started, err)
	return nil, ListOutput{Entries: entries}, err
}

func (s *Service) read(_ context.Context, _ *mcp.CallToolRequest, in ReadInput) (*mcp.CallToolResult, ReadOutput, error) {
	started := time.Now()
	file, err := s.fs.Read(in.Path, in.OffsetLine, in.MaxLines)
	s.record("read_file", in.Path, 0, file.BytesRead, started, err)
	return nil, ReadOutput{File: file}, err
}

func (s *Service) stat(_ context.Context, _ *mcp.CallToolRequest, in StatInput) (*mcp.CallToolResult, StatOutput, error) {
	started := time.Now()
	entry, err := s.fs.Stat(in.Path)
	s.record("stat_path", in.Path, 0, 0, started, err)
	return nil, StatOutput{Entry: entry}, err
}

func (s *Service) search(ctx context.Context, _ *mcp.CallToolRequest, in SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
	started := time.Now()
	path := defaultPath(in.Path)
	matches, scanned, err := s.fs.Search(ctx, in.Query, path, in.Regex, in.Limit)
	results := make([]SearchResult, 0, len(matches))
	for _, match := range matches {
		results = append(results, SearchResult{ID: match.ID, Title: match.Title, Text: match.Preview, Path: match.Path, Line: match.Line})
	}
	detail := "query_sha256=" + shortHash(in.Query) + fmt.Sprintf(" scanned=%d", scanned)
	s.recordWithDetail("search", path, len(results), 0, started, err, detail)
	return nil, SearchOutput{Results: results, ScannedFiles: scanned}, err
}

func (s *Service) fetch(_ context.Context, _ *mcp.CallToolRequest, in FetchInput) (*mcp.CallToolResult, FetchOutput, error) {
	started := time.Now()
	file, err := s.fs.Read(in.ID, 1, 5000)
	s.record("fetch", in.ID, 0, file.BytesRead, started, err)
	return nil, FetchOutput{ID: file.Path, Title: baseName(file.Path), Text: file.Content, Path: file.Path, Truncated: file.Truncated}, err
}

func (s *Service) Handler() http.Handler {
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		mode := "read-only"
		if s.cfg.HasWriteTools() {
			mode = "read-write"
		}
		_, _ = fmt.Fprintf(w, `{"status":"ok","mode":%q}`, mode)
	})
	mux.Handle("/mcp", s.guard(mcpHandler))
	return securityHeaders(mux)
}

// Run serves MCP over the supplied transport. Stdio mode lets local clients
// launch the server on demand without opening a TCP port.
func (s *Service) Run(ctx context.Context, transport mcp.Transport) error {
	return s.server.Run(ctx, transport)
}

func (s *Service) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remote := remoteIP(r)
		remoteHash := shortHash(remote)
		if !s.limiter.Allow(remote) {
			s.audit.Write(audit.Event{Event: "http", Outcome: "rate_limited", RemoteHash: remoteHash})
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		provided := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if len(provided) != len(s.cfg.Token) || subtle.ConstantTimeCompare([]byte(provided), []byte(s.cfg.Token)) != 1 {
			s.audit.Write(audit.Event{Event: "auth", Outcome: "denied", RemoteHash: remoteHash})
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		select {
		case s.sem <- struct{}{}:
			defer func() { <-s.sem }()
		case <-r.Context().Done():
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next.ServeHTTP(w, r)
	})
}

func (s *Service) record(tool, path string, results, bytes int, started time.Time, err error) {
	s.recordWithDetail(tool, path, results, bytes, started, err, "")
}

func (s *Service) recordWithDetail(tool, path string, results, bytes int, started time.Time, err error, detail string) {
	outcome := "ok"
	if err != nil {
		outcome = "denied_or_error"
		detail = strings.TrimSpace(detail + " error=" + sanitizeError(err))
	}
	s.audit.Write(audit.Event{Event: "tool", Tool: tool, Path: filepathForAudit(path), Outcome: outcome, Bytes: bytes, Results: results, DurationMS: time.Since(started).Milliseconds(), Detail: detail})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func defaultPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return "."
	}
	return path
}

func baseName(path string) string {
	path = strings.TrimSuffix(strings.ReplaceAll(path, "\\", "/"), "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

func filepathForAudit(path string) string {
	path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	if path == "" {
		return "."
	}
	return path
}

func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	if len(text) > 180 {
		text = text[:180]
	}
	return strings.ReplaceAll(text, "\n", " ")
}

func boolPtr(v bool) *bool { return &v }

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

type rateLimiter struct {
	mu     sync.Mutex
	limit  int
	counts map[string]*rateWindow
}

type rateWindow struct {
	start time.Time
	count int
}

func newRateLimiter(limit int) *rateLimiter {
	return &rateLimiter{limit: limit, counts: make(map[string]*rateWindow)}
}

func (r *rateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	w, ok := r.counts[key]
	if !ok || now.Sub(w.start) >= time.Minute {
		r.counts[key] = &rateWindow{start: now, count: 1}
		return true
	}
	if w.count >= r.limit {
		return false
	}
	w.count++
	if len(r.counts) > 1024 {
		for k, item := range r.counts {
			if now.Sub(item.start) >= time.Minute {
				delete(r.counts, k)
			}
		}
	}
	return true
}
