package safefs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wangchuncheng18/safe-local-files-mcp/internal/config"
)

func TestWritePolicyAndContainment(t *testing.T) {
	root := t.TempDir()
	cfg := config.Defaults()
	cfg.Root = root
	cfg.SearchTime = time.Second
	fs, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.WriteFile("note.txt", "hello", false); err == nil {
		t.Fatal("default read-only policy permitted write")
	}

	cfg.WritePermissions = config.WritePermissions{Enabled: true, CreateFiles: true, OverwriteFiles: true, CreateDirectories: true}
	fs, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.WriteFile("../escape.txt", "hello", false); err == nil {
		t.Fatal("traversal permitted")
	}
	if _, err := fs.WriteFile(".env", "hello", false); err == nil {
		t.Fatal("denied filename permitted")
	}
	if _, err := fs.WriteFile("secret.txt", "password=abcdefghijklmnopqrstuvwxyz123456", false); err == nil {
		t.Fatal("secret-like content permitted")
	}
	if _, err := fs.WriteFile("note.txt", "hello", false); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.WriteFile("note.txt", "replacement", false); err == nil {
		t.Fatal("create replaced existing file")
	}
	if _, err := fs.WriteFile("note.txt", "replacement", true); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "note.txt"))
	if err != nil || string(b) != "replacement" {
		t.Fatalf("overwrite result=%q err=%v", b, err)
	}
	if _, err := fs.CreateDirectory("nested"); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.WriteFile("nested/file.md", "text", false); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.CreateDirectory("nested"); err == nil {
		t.Fatal("existing directory was recreated")
	}
}

func TestWriteRespectsSizeAndSymlinks(t *testing.T) {
	root := t.TempDir()
	cfg := config.Defaults()
	cfg.Root = root
	cfg.SearchTime = time.Second
	cfg.MaxWriteBytes = 4
	cfg.WritePermissions = config.WritePermissions{Enabled: true, CreateFiles: true}
	fs, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.WriteFile("large.txt", "12345", false); err == nil {
		t.Fatal("oversized write permitted")
	}
	if _, err := fs.WriteFile("binary.txt", "a\x00b", false); err == nil {
		t.Fatal("binary write permitted")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err == nil {
		if _, err := fs.WriteFile("link/escape.txt", "test", false); err == nil {
			t.Fatal("symlink escape permitted")
		}
		if _, err := os.Stat(filepath.Join(outside, "escape.txt")); !os.IsNotExist(err) {
			t.Fatal("outside file created")
		}
	}
	if _, err := fs.WriteFile("bad.exe", "test", false); err == nil || !strings.Contains(err.Error(), "extension") {
		t.Fatalf("extension policy error=%v", err)
	}
}
