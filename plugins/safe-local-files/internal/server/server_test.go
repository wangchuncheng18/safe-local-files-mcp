package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wangchuncheng18/safe-local-files-mcp/internal/config"
)

type authTransport struct {
	token string
	base  http.RoundTripper
}

func (a authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+a.token)
	return a.base.RoundTrip(clone)
}

func TestHTTPAuthAndMCPToolCall(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello from the safe root"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Root = root
	cfg.Token = "0123456789abcdefghijklmnopqrstuvwxyz-TEST"
	cfg.AuditLog = filepath.Join(t.TempDir(), "audit.jsonl")
	cfg.SearchTime = time.Second
	service, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(service.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/mcp", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	httpClient := &http.Client{Transport: authTransport{token: cfg.Token, base: http.DefaultTransport}}
	transport := &mcp.StreamableClientTransport{Endpoint: ts.URL + "/mcp", HTTPClient: httpClient, DisableStandaloneSSE: true, MaxRetries: -1}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.1.0"}, nil)
	session, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "read_file", Arguments: map[string]any{"path": "hello.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %+v", result.Content)
	}
}

func TestWriteToolsAppearOnlyWhenEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		cfg := config.Defaults()
		cfg.Root = t.TempDir()
		cfg.AuditLog = filepath.Join(t.TempDir(), "audit.jsonl")
		cfg.SearchTime = time.Second
		cfg.WritePermissions = config.WritePermissions{Enabled: enabled, CreateFiles: true, CreateDirectories: true}
		service, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		serverTransport, clientTransport := mcp.NewInMemoryTransports()
		ctx, cancel := context.WithCancel(context.Background())
		serverDone := make(chan error, 1)
		go func() { serverDone <- service.Run(ctx, serverTransport) }()
		client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.1.0"}, nil)
		session, err := client.Connect(ctx, clientTransport, nil)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		list, err := session.ListTools(ctx, nil)
		if err != nil {
			session.Close()
			cancel()
			t.Fatal(err)
		}
		foundWrite, foundMkdir := false, false
		for _, tool := range list.Tools {
			if tool.Name == "write_file" {
				foundWrite = true
			}
			if tool.Name == "create_directory" {
				foundMkdir = true
			}
		}
		if foundWrite != enabled || foundMkdir != enabled {
			t.Errorf("enabled=%v: write=%v mkdir=%v", enabled, foundWrite, foundMkdir)
		}
		_ = session.Close()
		cancel()
		<-serverDone
	}
}

func TestCloudflareAccessModeDoesNotAcceptStaticToken(t *testing.T) {
	cfg := config.Defaults()
	cfg.Root = t.TempDir()
	cfg.AuditLog = filepath.Join(t.TempDir(), "audit.jsonl")
	cfg.AuthMode = "cloudflare_access"
	cfg.CloudflareTeamDomain = "https://team.cloudflareaccess.com"
	cfg.CloudflareAudience = "0123456789abcdef"
	cfg.Token = "0123456789abcdefghijklmnopqrstuvwxyz-TEST"
	service, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	request.Header.Set("Authorization", "Bearer "+cfg.Token)
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("static token accepted in Cloudflare mode: %d", recorder.Code)
	}
}
