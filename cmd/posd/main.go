// Command posd is the single Tatsy POS binary: HTTP server, SQLite store
// and the embedded UI. Run it on Terminal A; Terminal B just opens a browser.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tatsy/internal/api"
	"tatsy/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "pos.db", "sqlite database file")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	db, err := store.Open(*dbPath)
	if err != nil {
		slog.Error("open db", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()
	if err := store.Migrate(ctx, db); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}
	if err := store.Seed(ctx, db); err != nil {
		slog.Error("seed", "err", err)
		os.Exit(1)
	}

	hub := api.NewHub()
	srv, err := api.NewServer(db, hub)
	if err != nil {
		slog.Error("server", "err", err)
		os.Exit(1)
	}

	ip := lanIP()
	slog.Info("tatsy POS server running", "addr", *addr, "db", *dbPath)
	if ip != "" {
		slog.Info("open the till on this PC", "url", "http://localhost:"+port(*addr))
		slog.Info("open the buyer display on Terminal B", "url", "http://"+ip+":"+port(*addr)+"/display")
	}

	httpSrv := &http.Server{Addr: *addr, Handler: srv.Routes()}

	// Graceful shutdown: stop accepting, drain in-flight requests, close db.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown", "err", err)
		}
	}()

	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}
	if err := db.Close(); err != nil {
		slog.Error("close db", "err", err)
	}
	slog.Info("server stopped")
}

// lanIP finds the first non-loopback IPv4 address to print a helpful URL.
func lanIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	ip, _, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return ""
	}
	return ip
}

func port(addr string) string {
	if _, p, err := net.SplitHostPort(addr); err == nil && p != "" {
		return p
	}
	return "8080"
}
