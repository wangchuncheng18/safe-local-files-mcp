package safefs

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wangchuncheng18/safe-local-files-mcp/internal/config"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`),
	regexp.MustCompile(`(?i)(api[_-]?key|access[_-]?token|client[_-]?secret|password)\s*[:=]\s*["']?[A-Za-z0-9_./+=-]{20,}`),
}

type FS struct {
	cfg        config.Config
	root       string
	extensions map[string]struct{}
}

type Entry struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	Size     int64     `json:"size,omitempty"`
	Modified time.Time `json:"modified"`
}

type File struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	BytesRead int    `json:"bytes_read"`
	Truncated bool   `json:"truncated"`
}

type Match struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Title   string `json:"title"`
	Line    int    `json:"line,omitempty"`
	Preview string `json:"preview,omitempty"`
}

func New(cfg config.Config) (*FS, error) {
	root := cfg.Root
	if cfg.FollowSymlinks {
		resolved, err := filepath.EvalSymlinks(root)
		if err != nil {
			return nil, fmt.Errorf("resolve root symlink: %w", err)
		}
		root = resolved
	} else {
		info, err := os.Lstat(root)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("configured root may not be a symbolic link when follow_symlinks=false")
		}
		resolved, err := filepath.EvalSymlinks(root)
		if err != nil {
			return nil, fmt.Errorf("resolve root: %w", err)
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(root))
		if err != nil {
			return nil, fmt.Errorf("resolve root parent: %w", err)
		}
		if !samePath(filepath.Join(parent, filepath.Base(root)), resolved) {
			return nil, errors.New("configured root may not be a junction or symbolic link when follow_symlinks=false")
		}
		root = resolved
	}
	exts := make(map[string]struct{}, len(cfg.AllowExtensions))
	for _, ext := range cfg.AllowExtensions {
		if ext != "" {
			exts[strings.ToLower(ext)] = struct{}{}
		}
	}
	return &FS{cfg: cfg, root: filepath.Clean(root), extensions: exts}, nil
}

func (f *FS) Resolve(rel string, wantDir bool) (string, string, error) {
	if strings.ContainsRune(rel, 0) {
		return "", "", errors.New("path contains a null byte")
	}
	rel = filepath.Clean(filepath.FromSlash(strings.TrimSpace(rel)))
	if rel == "" || rel == "." {
		rel = "."
	}
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", errors.New("path must stay inside the configured root")
	}
	if f.denied(rel) {
		return "", "", errors.New("path is denied by policy")
	}
	abs := filepath.Join(f.root, rel)
	contained, err := within(f.root, abs)
	if err != nil || !contained {
		return "", "", errors.New("path escapes the configured root")
	}
	if f.cfg.FollowSymlinks {
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return "", "", err
		}
		contained, err = within(f.root, resolved)
		if err != nil || !contained {
			return "", "", errors.New("resolved path escapes the configured root")
		}
		abs = resolved
	} else if err := rejectSymlinks(f.root, rel); err != nil {
		return "", "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", err
	}
	if wantDir && !info.IsDir() {
		return "", "", errors.New("path is not a directory")
	}
	if !wantDir && info.IsDir() {
		return "", "", errors.New("path is a directory")
	}
	return abs, filepath.ToSlash(rel), nil
}

func (f *FS) Stat(rel string) (Entry, error) {
	abs, clean, err := f.Resolve(rel, false)
	if err != nil {
		if dirAbs, dirClean, dirErr := f.Resolve(rel, true); dirErr == nil {
			abs, clean, err = dirAbs, dirClean, nil
		}
	}
	if err != nil {
		return Entry{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Entry{}, err
	}
	typeName := "file"
	if info.IsDir() {
		typeName = "directory"
	}
	return Entry{Path: clean, Name: info.Name(), Type: typeName, Size: info.Size(), Modified: info.ModTime().UTC()}, nil
}

func (f *FS) List(rel string, depth, limit int) ([]Entry, error) {
	if depth < 1 {
		depth = 1
	}
	if depth > 5 {
		depth = 5
	}
	if limit < 1 || limit > 1000 {
		limit = 200
	}
	abs, clean, err := f.Resolve(rel, true)
	if err != nil {
		return nil, err
	}
	baseDepth := strings.Count(filepath.Clean(abs), string(filepath.Separator))
	entries := make([]Entry, 0, min(limit, 64))
	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return filepath.SkipDir
		}
		if path == abs {
			return nil
		}
		relPath, err := filepath.Rel(f.root, path)
		if err != nil {
			return nil
		}
		if f.denied(relPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 && !f.cfg.FollowSymlinks {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		currentDepth := strings.Count(filepath.Clean(path), string(filepath.Separator)) - baseDepth
		if currentDepth > depth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		typeName := "file"
		if d.IsDir() {
			typeName = "directory"
		}
		entries = append(entries, Entry{Path: filepath.ToSlash(relPath), Name: d.Name(), Type: typeName, Size: info.Size(), Modified: info.ModTime().UTC()})
		if len(entries) >= limit {
			return io.EOF
		}
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return strings.ToLower(entries[i].Path) < strings.ToLower(entries[j].Path) })
	_ = clean
	return entries, nil
}

func (f *FS) Read(rel string, offsetLine, maxLines int) (File, error) {
	abs, clean, err := f.Resolve(rel, false)
	if err != nil {
		return File{}, err
	}
	if !f.allowedExtension(abs) {
		return File{}, errors.New("file extension is not allowed")
	}
	info, err := os.Stat(abs)
	if err != nil {
		return File{}, err
	}
	if info.Size() > f.cfg.MaxFileBytes {
		return File{}, fmt.Errorf("file exceeds max_file_bytes (%d)", f.cfg.MaxFileBytes)
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return File{}, err
	}
	if bytes.IndexByte(b, 0) >= 0 {
		return File{}, errors.New("binary files are not readable")
	}
	text := string(b)
	if containsSecret(text) {
		if f.cfg.SensitiveContentAction == "deny" {
			return File{}, errors.New("file contains secret-like content and was denied")
		}
		text = redactSecrets(text)
	}
	lines := strings.Split(text, "\n")
	if offsetLine < 1 {
		offsetLine = 1
	}
	if maxLines < 1 || maxLines > 5000 {
		maxLines = 500
	}
	start := min(offsetLine-1, len(lines))
	end := min(start+maxLines, len(lines))
	out := strings.Join(lines[start:end], "\n")
	truncated := end < len(lines)
	if len(out) > f.cfg.MaxResponseBytes {
		out = truncateUTF8(out, f.cfg.MaxResponseBytes)
		truncated = true
	}
	return File{Path: clean, Content: out, BytesRead: len(out), Truncated: truncated}, nil
}

func (f *FS) Search(ctx context.Context, query, rel string, regex bool, limit int) ([]Match, int, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, 0, errors.New("query is required")
	}
	if len(query) > 512 {
		return nil, 0, errors.New("query is too long")
	}
	if limit < 1 || limit > f.cfg.MaxSearchResults {
		limit = f.cfg.MaxSearchResults
	}
	abs, _, err := f.Resolve(rel, true)
	if err != nil {
		return nil, 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, f.cfg.SearchTime)
	defer cancel()
	var re *regexp.Regexp
	if regex {
		re, err = regexp.Compile(query)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid regular expression: %w", err)
		}
	}
	needle := strings.ToLower(query)
	results := make([]Match, 0, min(limit, 32))
	scanned := 0
	stop := errors.New("stop")
	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, walkErr error) error {
		select {
		case <-ctx.Done():
			return stop
		default:
		}
		if walkErr != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == abs {
			return nil
		}
		relPath, relErr := filepath.Rel(f.root, path)
		if relErr != nil || f.denied(relPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 && !f.cfg.FollowSymlinks {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !f.allowedExtension(path) {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil || info.Size() > f.cfg.MaxFileBytes {
			return nil
		}
		scanned++
		if scanned > f.cfg.MaxSearchFiles {
			return stop
		}
		displayPath := filepath.ToSlash(relPath)
		nameMatch := re != nil && re.MatchString(d.Name()) || re == nil && strings.Contains(strings.ToLower(d.Name()), needle)
		if nameMatch {
			results = append(results, Match{ID: displayPath, Path: displayPath, Title: d.Name(), Preview: "filename match"})
			if len(results) >= limit {
				return stop
			}
		}
		file, openErr := os.Open(path)
		if openErr != nil {
			return nil
		}
		scanner := bufio.NewScanner(io.LimitReader(file, f.cfg.MaxFileBytes+1))
		buf := make([]byte, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			matched := re != nil && re.MatchString(line) || re == nil && strings.Contains(strings.ToLower(line), needle)
			if !matched {
				continue
			}
			preview := strings.TrimSpace(line)
			if containsSecret(preview) {
				preview = "[REDACTED: secret-like content]"
			}
			if len(preview) > 500 {
				preview = preview[:500] + "…"
			}
			results = append(results, Match{ID: displayPath, Path: displayPath, Title: d.Name(), Line: lineNo, Preview: preview})
			if len(results) >= limit {
				_ = file.Close()
				return stop
			}
		}
		_ = file.Close()
		return nil
	})
	if err != nil && !errors.Is(err, stop) {
		return nil, scanned, err
	}
	return results, scanned, nil
}

func (f *FS) denied(rel string) bool {
	rel = filepath.ToSlash(filepath.Clean(rel))
	lower := strings.ToLower(rel)
	base := strings.ToLower(filepath.Base(rel))
	for _, pattern := range f.cfg.DenyGlobs {
		p := strings.ToLower(filepath.ToSlash(pattern))
		if ok, _ := filepath.Match(filepath.FromSlash(p), filepath.FromSlash(lower)); ok {
			return true
		}
		if ok, _ := filepath.Match(filepath.FromSlash(p), filepath.FromSlash(base)); ok {
			return true
		}
		if !strings.Contains(p, "/") {
			for _, component := range strings.Split(lower, "/") {
				if ok, _ := filepath.Match(filepath.FromSlash(p), filepath.FromSlash(component)); ok {
					return true
				}
			}
		}
		trimmed := strings.TrimSuffix(p, "/**")
		if trimmed != p && (lower == trimmed || strings.HasPrefix(lower, trimmed+"/")) {
			return true
		}
	}
	return false
}

func (f *FS) allowedExtension(path string) bool {
	_, ok := f.extensions[strings.ToLower(filepath.Ext(path))]
	return ok
}

func rejectSymlinks(root, rel string) error {
	current := root
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(filepath.Clean(rel), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("symbolic links are disabled")
		}
	}
	return nil
}

func within(root, target string) (bool, error) {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false, err
	}
	if runtime.GOOS == "windows" {
		rel = strings.ToLower(rel)
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel), nil
}

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	b := []byte(s[:maxBytes])
	for len(b) > 0 && !utf8.Valid(b) {
		b = b[:len(b)-1]
	}
	return string(b)
}

func containsSecret(s string) bool {
	for _, pattern := range secretPatterns {
		if pattern.MatchString(s) {
			return true
		}
	}
	return false
}

func redactSecrets(s string) string {
	for _, pattern := range secretPatterns {
		s = pattern.ReplaceAllString(s, "[REDACTED]")
	}
	return s
}
