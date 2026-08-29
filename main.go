// Command escalight runs the Escalight server: a single self-contained
// binary providing on-call scheduling, escalation policies, alert ingestion,
// and paging over email, Slack, Discord, and web push.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Laaaaksh/escalight/internal/config"
	"github.com/Laaaaksh/escalight/internal/db"
	"github.com/Laaaaksh/escalight/internal/engine"
	"github.com/Laaaaksh/escalight/internal/httpserver"
	"github.com/Laaaaksh/escalight/internal/notify"
	"github.com/Laaaaksh/escalight/internal/version"
	"github.com/Laaaaksh/escalight/internal/webhooks"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Println("escalight " + version.Version)
			return
		case "serve":
			// default and only real subcommand; falls through below
		default:
			fmt.Fprintf(os.Stderr, "usage: escalight [serve|version]\n")
			os.Exit(1)
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg := config.Load()

	conn, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer conn.Close()
	store := db.NewStore(conn)

	vapidPub, vapidPriv, err := notify.EnsureVAPIDKeys(store, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey)
	if err != nil {
		return fmt.Errorf("ensure VAPID keys: %w", err)
	}

	dispatcher := &notify.Dispatcher{
		Store:   store,
		BaseURL: cfg.BaseURL,
		Email:   &notify.EmailSender{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom},
		Slack:   &notify.SlackSender{WebhookURL: cfg.SlackWebhookURL},
		Discord: &notify.DiscordSender{WebhookURL: cfg.DiscordWebhookURL},
		Push:    &notify.WebPushSender{PublicKey: vapidPub, PrivateKey: vapidPriv, Subject: cfg.VAPIDSubject},
		Logger:  logger,
	}

	eng := engine.New(store, dispatcher, logger)
	ingestor := &webhooks.Ingestor{Store: store, Engine: eng, Logger: logger}

	srv, err := httpserver.New(store, eng, dispatcher, ingestor, cfg, vapidPub, logger)
	if err != nil {
		return fmt.Errorf("build server: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go eng.Run(ctx)

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		httpSrv.Shutdown(shutdownCtx)
	}()

	logger.Info("escalight starting", "addr", cfg.Addr, "base_url", cfg.BaseURL, "db", cfg.DBPath, "version", version.Version)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
