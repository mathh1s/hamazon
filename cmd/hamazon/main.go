package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mathh1s/hamazon/internal/catalog"
	"github.com/mathh1s/hamazon/internal/config"
	"github.com/mathh1s/hamazon/internal/db"
	"github.com/mathh1s/hamazon/internal/server"
	"github.com/mathh1s/hamazon/web"
)

const usage = `hamazon, a private Minecraft item shop

usage:
  hamazon serve       run the web server (default)
`

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	var err error
	switch cmd {
	case "serve":
		err = serve(log)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		log.Error(cmd+" failed", "err", err)
		os.Exit(1)
	}
}

func serve(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conn, err := openDatabase(ctx, cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	items := catalog.NewStore(conn)
	if err := seedCatalog(ctx, cfg, items); err != nil {
		return err
	}

	srv := server.New(cfg, log, web.Dist())
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.Addr)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func openDatabase(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	conn, err := db.Open(cfg.DatabasePath())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Migrate(ctx, conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func seedCatalog(ctx context.Context, cfg config.Config, items *catalog.Store) error {
	seed := catalog.DefaultSeed
	if cfg.SeedFile != "" {
		custom, err := os.ReadFile(cfg.SeedFile)
		if err != nil {
			return fmt.Errorf("read seed file: %w", err)
		}
		seed = custom
	}
	return items.Seed(ctx, seed)
}
