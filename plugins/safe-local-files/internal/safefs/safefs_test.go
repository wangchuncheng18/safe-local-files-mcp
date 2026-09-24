package safefs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wangchuncheng18/safe-local-files-mcp/internal/config"
)

func testFS(t *testing.T, action string) (*FS, string) {
	t.Helper()
	root := t.TempDir()
	cfg := config.Defaults()
	cfg.Root = root
	cfg.SearchTime = 2 * time.Second
	cfg.SensitiveContentAction = action
	fs, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return fs, root
}

func TestReadDeniesTraversalPolicyAndSecrets(t *testing.T) {
	fs, root := testFS(t, "deny")
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("public information\nsecond line"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SAFE_LOCAL_FILES_TOKEN=not-for-reading"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "settings.txt"), []byte("api_key=abcdefghijklmnopqrstuvwxyz123456"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "nested", ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", ".git", "config"), []byte("repository metadata"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := fs.Read("notes.txt", 1, 10)
	if err != nil || !strings.Contains(got.Content, "public information") {
		t.Fatalf("read normal file: got=%+v err=%v", got, err)
	}
	if _, err := fs.Read("../outside.txt", 1, 10); err == nil {
		t.Fatal("path traversal was not denied")
	}
	if _, err := fs.Read(".env", 1, 10); err == nil {
		t.Fatal("denied filename was readable")
	}
	if _, err := fs.Read("settings.txt", 1, 10); err == nil || !strings.Contains(err.Error(), "secret-like") {
		t.Fatalf("secret content was not denied: %v", err)
	}
	if _, err := fs.Read("nested/.git/config", 1, 10); err == nil {
		t.Fatal("nested denied directory was readable")
	}
}

func TestSearchRedactsSecretLikeMatchingLine(t *testing.T) {
	fs, root := testFS(t, "redact")
	if err := os.WriteFile(filepath.Join(root, "report.txt"), []byte("status is green\npassword=abcdefghijklmnopqrstuvwxyz123456"), 0o600); err != nil {
		t.Fatal(err)
	}
	matches, scanned, err := fs.Search(context.Background(), "password", ".", false, 10)
	if err != nil {
		t.Fatal(err)
	}
	if scanned != 1 || len(matches) != 1 {
		t.Fatalf("unexpected search results: scanned=%d matches=%+v", scanned, matches)
	}
	if matches[0].Preview != "[REDACTED: secret-like content]" {
		t.Fatalf("secret preview was exposed: %q", matches[0].Preview)
	}
}

func TestRootAllowsSymlinkInAncestorButNotAtRoot(t *testing.T) {
	base := t.TempDir()
	realParent := filepath.Join(base, "real-parent")
	root := filepath.Join(realParent, "approved")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(realParent, alias); err != nil {
		t.Skipf("symlinks unavailable on this runner: %v", err)
	}
	cfg := config.Defaults()
	cfg.Root = filepath.Join(alias, "approved")
	fs, err := New(cfg)
	if err != nil {
		t.Fatalf("ancestor symlink rejected: %v", err)
	}
	if _, err := fs.Stat("."); err != nil {
		t.Fatalf("approved root unavailable: %v", err)
	}
	cfg.Root = alias
	if _, err := New(cfg); err == nil {
		t.Fatal("symlink used as the root was accepted")
	}
}
