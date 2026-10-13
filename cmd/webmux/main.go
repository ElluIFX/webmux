package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/rainchan/webmux/internal/api"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/session"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println("webmux", version)
		return
	}
	if err := run(); err != nil {
		slog.Error("webmux failed", "error", err)
		os.Exit(1)
	}
}
func run() error {
	flags := flag.NewFlagSet("webmux", flag.ContinueOnError)
	path := flags.String("config", config.DefaultPath(), "TOML configuration path")
	listen := flags.String("listen", "", "HTTP listen address")
	token := flags.String("token", "", "optional shared token")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	explicit := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	c, err := config.Load(*path, explicit["config"])
	if err != nil {
		return err
	}
	if explicit["listen"] {
		c.Listen = *listen
	}
	if explicit["token"] {
		c.Token = *token
	}
	if err = c.Validate(); err != nil {
		return err
	}
	host, _, _ := net.SplitHostPort(c.Listen)
	if (host == "0.0.0.0" || host == "::" || host == "") && c.Token == "" {
		slog.Warn("SHELL EXPOSED: listening on all interfaces without token; isolate with Tailscale/WireGuard")
	}
	m := session.NewManager(c.SessionOptions())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	listener, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: api.New(c, m), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	ended := make(chan error, 1)
	go func() { ended <- server.Serve(listener) }()
	slog.Info("webmux listening", "address", listener.Addr(), "version", version)
	select {
	case <-ctx.Done():
	case err = <-ended:
		if err != http.ErrServerClosed {
			return err
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	m.Shutdown(shutdown)
	return server.Shutdown(shutdown)
}
