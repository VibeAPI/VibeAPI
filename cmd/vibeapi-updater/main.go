package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/QuantumNous/new-api/internal/systemupdater"
	"github.com/QuantumNous/new-api/pkg/systemupdate"
)

func main() {
	configPath := flag.String("config", "/etc/vibeapi-updater/config.json", "absolute updater configuration path")
	flag.Parse()

	config, err := systemupdater.LoadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	catalog := systemupdate.NewCatalog(systemupdate.CatalogConfig{
		ReleaseRepository: config.ReleaseRepository,
		ImageRepository:   config.ImageRepository,
		CacheTTL:          5 * time.Minute,
		RequestTimeout:    15 * time.Second,
	}, nil)
	engine := systemupdater.NewEngine(
		config,
		systemupdater.NewReleaseCatalog(catalog, config.ImageRepository),
		systemupdater.NewExecRunner(config.APIToken, config.ReadinessToken),
	)
	if err := engine.RecoverInterrupted(context.Background()); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(config.SocketPath), 0750); err != nil {
		log.Fatal(err)
	}
	if err := os.Remove(config.SocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatal(err)
	}
	listener, err := net.Listen("unix", config.SocketPath)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	defer os.Remove(config.SocketPath)
	if err := os.Chmod(config.SocketPath, 0660); err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Handler:           systemupdater.NewHandler(engine, config.APIToken),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 * 1024,
	}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Serve(listener) }()

	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
		return
	case <-shutdownCtx.Done():
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), config.ShutdownWaitTimeoutDuration)
	defer cancel()
	if err := server.Shutdown(waitCtx); err != nil {
		log.Printf("HTTP shutdown: %v", err)
	}
	if err := engine.Wait(waitCtx); err != nil {
		log.Fatal(fmt.Errorf("wait for update operation: %w", err))
	}
}
