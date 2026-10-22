package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/mathh1s/hamazon/internal/assets"
	"github.com/mathh1s/hamazon/internal/auth"
	"github.com/mathh1s/hamazon/internal/catalog"
	"github.com/mathh1s/hamazon/internal/config"
	"github.com/mathh1s/hamazon/internal/db"
	"github.com/mathh1s/hamazon/internal/discord"
	"github.com/mathh1s/hamazon/internal/orders"
	"github.com/mathh1s/hamazon/internal/server"
	"github.com/mathh1s/hamazon/web"
)

const usage = `hamazon, a private Minecraft item shop

usage:
  hamazon serve                       run the web server (default)
  hamazon extract [--jar client.jar]  render item icons, GUI sprites and the
                                      font; downloads the client jar of
                                      HAMAZON_MC_VERSION unless --jar is given
  hamazon admin add <name>            create an admin (asks for a password)
  hamazon admin passwd <name>         change an admin password
  hamazon admin remove <name>         delete an admin
  hamazon admin list                  list admins
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
	case "admin":
		err = adminCommand(os.Args[2:])
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
	seed, err := seedCatalog(ctx, cfg, items)
	if err != nil {
		return err
	}

	var notifier *discord.Notifier
	orderService := orders.NewService(conn, items, cfg.SessionSecret, cfg.FloodgatePrefix, func() { notifier.Wake() })
	notifier = discord.NewNotifier(cfg.WebhookURL, cfg.PublicURL, orderService, items, log)
	if cfg.WebhookURL == "" {
		log.Warn("HAMAZON_DISCORD_WEBHOOK_URL is not set, orders are saved but no Discord messages are sent")
	}
	go notifier.Run(ctx)

	assetManager := assets.NewManager(cfg.AssetsDir(), cfg.MCVersion, log)
	if cfg.AutoAssets {
		go assetManager.Run(ctx)
	} else {
		assetManager.Disable()
	}

	srv := server.New(cfg, log, server.Deps{
		Catalog:  items,
		Orders:   orderService,
		Access:   auth.NewAccess(cfg.ShopPassword, cfg.SessionSecret),
		Admins:   auth.NewAdmins(conn),
		Assets:   assetManager,
		Defaults: seed,
		Frontend: web.Dist(),
	})
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

func seedCatalog(ctx context.Context, cfg config.Config, items *catalog.Store) ([]byte, error) {
	seed := catalog.DefaultSeed
	if cfg.SeedFile != "" {
		custom, err := os.ReadFile(cfg.SeedFile)
		if err != nil {
			return nil, fmt.Errorf("read seed file: %w", err)
		}
		seed = custom
	}
	return seed, items.Seed(ctx, seed)
}

func extract(log *slog.Logger, args []string) error {
	cfg := config.LoadBase()
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	jar := fs.String("jar", "", "path to a Minecraft client jar, downloaded from Mojang when empty")
	version := fs.String("version", cfg.MCVersion, "Minecraft version to download when --jar is empty")
	out := fs.String("out", cfg.AssetsDir(), "output directory")
	verbose := fs.Bool("v", false, "list every item that could not be rendered")
	if err := fs.Parse(args); err != nil {
		return err
	}
	start := time.Now()
	if *jar == "" {
		log.Info("downloading client jar", "version", *version)
		path, err := assets.DownloadClientJar(context.Background(), &http.Client{Timeout: 10 * time.Minute}, *version, os.TempDir())
		if err != nil {
			return err
		}
		defer os.Remove(path)
		*jar = path
	}

	sounds, err := assets.DownloadSounds(context.Background(), &http.Client{Timeout: time.Minute}, *version)
	if err != nil {
		log.Warn("could not download sounds, continuing without", "err", err)
	}
	report, err := assets.Extract(*jar, *out, sounds)
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

func adminCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: hamazon admin add|passwd|remove|list [name]")
	}
	ctx := context.Background()
	conn, err := openDatabase(ctx, config.LoadBase())
	if err != nil {
		return err
	}
	defer conn.Close()
	admins := auth.NewAdmins(conn)

	if args[0] == "list" {
		names, err := admins.List(ctx)
		if err != nil {
			return err
		}
		if len(names) == 0 {
			fmt.Println("no admins yet, create one with: hamazon admin add <name>")
		}
		for _, n := range names {
			fmt.Println(n)
		}
		return nil
	}
	if len(args) != 2 {
		return errors.New("usage: hamazon admin " + args[0] + " <name>")
	}
	name := args[1]

	switch args[0] {
	case "add", "passwd":
		password, err := readPassword()
		if err != nil {
			return err
		}
		if args[0] == "add" {
			err = admins.Create(ctx, name, password)
		} else {
			err = admins.SetPassword(ctx, name, password)
		}
		if err != nil {
			return err
		}
		fmt.Println("saved admin", name)
	case "remove":
		if err := admins.Remove(ctx, name); err != nil {
			return err
		}
		fmt.Println("removed admin", name)
	default:
		return errors.New("unknown admin command " + args[0])
	}
	return nil
}

// readPassword prompts twice without echo on a terminal, or reads one line
// from stdin when piped (for scripted setups).
func readPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", errors.New("no password on stdin")
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Fprint(os.Stderr, "Password: ")
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "Repeat password: ")
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(first) != string(second) {
		return "", errors.New("passwords do not match")
	}
	return string(first), nil
}
