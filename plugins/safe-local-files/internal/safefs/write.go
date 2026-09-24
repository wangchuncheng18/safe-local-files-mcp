package safefs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// WriteFile creates or replaces an allowed text file. The caller must select
// the exact operation; neither mode grants the other.
func (f *FS) WriteFile(rel, content string, overwrite bool) (Entry, error) {
	permissions := f.cfg.WritePermissions
	if !permissions.Enabled || (overwrite && !permissions.OverwriteFiles) || (!overwrite && !permissions.CreateFiles) {
		return Entry{}, errors.New("file write is disabled by policy")
	}
	if len(content) > int(f.cfg.MaxWriteBytes) {
		return Entry{}, errors.New("content exceeds max_write_bytes")
	}
	if !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return Entry{}, errors.New("only UTF-8 text content is allowed")
	}
	if containsSecret(content) {
		return Entry{}, errors.New("secret-like content is not writable")
	}
	clean, err := f.writePath(rel, false)
	if err != nil {
		return Entry{}, err
	}
	root, err := os.OpenRoot(f.root)
	if err != nil {
		return Entry{}, err
	}
	defer root.Close()
	if overwrite {
		// Preserve the read policy for the existing file: it must be a regular,
		// permitted file without detected secrets before it can be replaced.
		if _, err := f.Read(clean, 1, 1); err != nil {
			return Entry{}, fmt.Errorf("existing file is not eligible for overwrite: %w", err)
		}
		var nonce [12]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return Entry{}, err
		}
		temp := filepath.Join(filepath.Dir(clean), ".safe-local-files-"+hex.EncodeToString(nonce[:])+".tmp")
		out, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return Entry{}, err
		}
		defer root.Remove(temp)
		if err := writeAndSync(out, content); err != nil {
			return Entry{}, err
		}
		if err := root.Rename(temp, clean); err != nil {
			return Entry{}, err
		}
	} else {
		out, err := root.OpenFile(clean, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return Entry{}, err
		}
		if err := writeAndSync(out, content); err != nil {
			_ = root.Remove(clean)
			return Entry{}, err
		}
	}
	return f.Stat(clean)
}

func writeAndSync(out *os.File, content string) error {
	_, err := out.WriteString(content)
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func (f *FS) CreateDirectory(rel string) (Entry, error) {
	if !f.cfg.WritePermissions.Enabled || !f.cfg.WritePermissions.CreateDirectories {
		return Entry{}, errors.New("directory creation is disabled by policy")
	}
	clean, err := f.writePath(rel, true)
	if err != nil {
		return Entry{}, err
	}
	root, err := os.OpenRoot(f.root)
	if err != nil {
		return Entry{}, err
	}
	defer root.Close()
	if err := root.Mkdir(clean, 0o700); err != nil {
		return Entry{}, err
	}
	return f.Stat(clean)
}

func (f *FS) writePath(rel string, directory bool) (string, error) {
	if strings.ContainsRune(rel, 0) || strings.TrimSpace(rel) == "" {
		return "", errors.New("a relative path is required")
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || filepath.IsAbs(clean) || filepath.VolumeName(clean) != "" || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("path must stay inside the configured root")
	}
	if f.denied(clean) {
		return "", errors.New("path is denied by policy")
	}
	if !directory && !f.allowedExtension(clean) {
		return "", errors.New("file extension is not allowed")
	}
	if _, _, err := f.Resolve(filepath.Dir(clean), true); err != nil {
		return "", err
	}
	return clean, nil
}
