package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadForStdioDoesNotRequireHTTPToken(t *testing.T) {
	t.Setenv("SAFE_LOCAL_FILES_TOKEN", "")
	root := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.json")
	contents := `{"root":` + quote(root) + `,"audit_log":` + quote(filepath.Join(t.TempDir(), "audit.jsonl")) + `}`
	if err := os.WriteFile(configPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadForStdio(configPath); err != nil {
		t.Fatalf("LoadForStdio failed without an HTTP token: %v", err)
	}
	if _, err := Load(configPath); err == nil {
		t.Fatal("Load unexpectedly accepted a missing HTTP token")
	}
	remoteOnly := `{"root":` + quote(root) + `,"audit_log":` + quote(filepath.Join(t.TempDir(), "audit.jsonl")) + `,"listen":"192.0.2.10","write_permissions":{"enabled":true,"create_files":true}}`
	if err := os.WriteFile(configPath, []byte(remoteOnly), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadForStdio(configPath); err != nil {
		t.Fatalf("stdio applied network-only requirements: %v", err)
	}
}

func TestRemoteRequiresTLSAndSeparateWriteOptIn(t *testing.T) {
	t.Setenv("SAFE_LOCAL_FILES_TOKEN", "abcdefghijklmnopqrstuvwxyz0123456789")
	root := t.TempDir()
	configDir := t.TempDir()
	cert := filepath.Join(configDir, "cert.pem")
	key := filepath.Join(configDir, "key.pem")
	for _, path := range []string{cert, key} {
		if err := os.WriteFile(path, []byte("placeholder"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(configDir, "config.json")
	base := `{"root":` + quote(root) + `,"audit_log":` + quote(filepath.Join(configDir, "audit.jsonl")) + `,"listen":"192.0.2.10","allow_remote":true,"write_permissions":{"enabled":true,"create_files":true}`
	if err := os.WriteFile(configPath, []byte(base+`}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(configPath); err == nil {
		t.Fatal("remote listener accepted without TLS")
	}
	withTLS := base + `,"tls_cert_file":` + quote(cert) + `,"tls_key_file":` + quote(key)
	if err := os.WriteFile(configPath, []byte(withTLS+`}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(configPath); err == nil {
		t.Fatal("remote write accepted without separate opt-in")
	}
	if err := os.WriteFile(configPath, []byte(withTLS+`,"allow_remote_write":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(configPath); err != nil {
		t.Fatalf("remote TLS and write opt-in rejected: %v", err)
	}
}

func quote(value string) string {
	result, _ := json.Marshal(value)
	return string(result)
}
