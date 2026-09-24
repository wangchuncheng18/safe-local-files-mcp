package main

import (
	"testing"

	"github.com/wangchuncheng18/safe-local-files-mcp/internal/config"
)

func TestCheckCloudflareReadOnly(t *testing.T) {
	base := config.Defaults()
	base.AuthMode = "cloudflare_access"
	base.BehindProxy = true
	if err := checkCloudflareReadOnly(base); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*config.Config){
		"static token mode": func(c *config.Config) { c.AuthMode = "bearer_token" },
		"public listener":  func(c *config.Config) { c.Listen = "0.0.0.0" },
		"remote enabled":   func(c *config.Config) { c.AllowRemote = true },
		"write enabled": func(c *config.Config) {
			c.WritePermissions.Enabled = true
			c.WritePermissions.CreateFiles = true
		},
		"write master switch": func(c *config.Config) { c.WritePermissions.Enabled = true },
		"remote write enabled": func(c *config.Config) { c.AllowRemoteWrite = true },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := base
			mutate(&cfg)
			if checkCloudflareReadOnly(cfg) == nil {
				t.Fatal("unsafe Cloudflare configuration accepted")
			}
		})
	}
}
