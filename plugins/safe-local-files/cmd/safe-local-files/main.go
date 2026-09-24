package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wangchuncheng18/safe-local-files-mcp/internal/config"
	serverpkg "github.com/wangchuncheng18/safe-local-files-mcp/internal/server"
)

var version = "0.2.4"

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		serve(os.Args[2:])
	case "stdio":
		stdio(os.Args[2:])
	case "validate":
		validate(os.Args[2:])
	case "token":
		token()
	case "version", "--version", "-version":
		fmt.Println(version)
	default:
		usage()
		os.Exit(2)
	}
}

func stdio(args []string) {
	flags := flag.NewFlagSet("stdio", flag.ExitOnError)
	configPath := flags.String("config", envOr("SLFM_CONFIG", "config.json"), "path to JSON config")
	_ = flags.Parse(args)
	cfg, err := config.LoadForStdio(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	service, err := serverpkg.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	maxFrameBytes := int(cfg.MaxWriteBytes*6 + (64 << 10))
	if err := service.Run(context.Background(), &mcp.StdioTransport{MaxLineLength: maxFrameBytes}); err != nil {
		log.Fatal(err)
	}
}

func serve(args []string) {
	flags := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := flags.String("config", envOr("SLFM_CONFIG", "config.json"), "path to JSON config")
	_ = flags.Parse(args)
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	service, err := serverpkg.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	httpServer := &http.Server{
		Addr:              cfg.Address(),
		Handler:           service.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      65 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	errCh := make(chan error, 1)
	go func() {
		scheme := "http"
		if cfg.TLSCertFile != "" {
			scheme = "https"
		}
		log.Printf("safe-local-files %s listening on %s://%s/mcp (root is configured; value omitted)", version, scheme, cfg.Address())
		if cfg.TLSCertFile != "" {
			errCh <- httpServer.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			errCh <- httpServer.ListenAndServe()
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	select {
	case sig := <-sigCh:
		log.Printf("received %s; shutting down", sig)
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func validate(args []string) {
	flags := flag.NewFlagSet("validate", flag.ExitOnError)
	configPath := flags.String("config", envOr("SLFM_CONFIG", "config.json"), "path to JSON config")
	requireCloudflareReadOnly := flags.Bool("require-cloudflare-read-only", false, "require a loopback Cloudflare Access HTTP server with all write tools disabled")
	_ = flags.Parse(args)
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	if *requireCloudflareReadOnly {
		if err := checkCloudflareReadOnly(cfg); err != nil {
			log.Fatal(err)
		}
	}
	if _, err := serverpkg.New(cfg); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("valid: root is accessible, listen=%s, auth_mode=%s, audit logging is writable\n", cfg.Address(), cfg.AuthMode)
}

func checkCloudflareReadOnly(cfg config.Config) error {
	if cfg.AuthMode != "cloudflare_access" || !cfg.BehindProxy || cfg.Listen != "127.0.0.1" || cfg.AllowRemote || cfg.AllowRemoteWrite || cfg.WritePermissions.Enabled || cfg.HasWriteTools() {
		return fmt.Errorf("Cloudflare quick-chat setup requires auth_mode=cloudflare_access, 127.0.0.1, behind_proxy=true, and all remote/write flags disabled")
	}
	return nil
}

func token() {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatal(err)
	}
	fmt.Println(base64.RawURLEncoding.EncodeToString(b))
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: safe-local-files <serve|stdio|validate|token|version> [options]")
}
