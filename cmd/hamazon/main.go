package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mathh1s/hamazon/internal/assets"
	"github.com/mathh1s/hamazon/internal/catalog"
	"github.com/mathh1s/hamazon/internal/config"
	"github.com/mathh1s/hamazon/internal/db"
	"github.com/mathh1s/hamazon/internal/server"
	"github.com/mathh1s/hamazon/web"
)

const usage = `hamazon, a private Minecraft item shop

usage:
  hamazon serve                       run the web server (default)
  hamazon extract --jar client.jar    render item icons, GUI sprites and the
                                      font from a Minecraft client jar
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
	case "extract":
		err = extract(log, os.Args[2:])
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

func extract(log *slog.Logger, args []string) error {
	cfg := config.LoadBase()
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	jar := fs.String("jar", "", "path to the Minecraft client jar, e.g. versions/26.2/26.2.jar")
	out := fs.String("out", cfg.AssetsDir(), "output directory")
	verbose := fs.Bool("v", false, "list every item that could not be rendered")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *jar == "" {
		return errors.New("--jar is required")
	}

	start := time.Now()
	report, err := assets.Extract(*jar, *out)
	if err != nil {
		return err
	}
	log.Info("assets extracted", "version", report.Manifest.Version, "items", len(report.Manifest.Items),
		"skipped", len(report.Skipped), "font", report.Manifest.Font, "took", time.Since(start).Round(time.Millisecond))
	if *verbose {
		for id, reason := range report.Skipped {
			log.Info("skipped", "item", id, "reason", reason)
		}
	}
	return nil
}
