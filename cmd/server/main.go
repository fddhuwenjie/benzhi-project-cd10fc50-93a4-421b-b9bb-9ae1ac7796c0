package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"timber-pest-remediation-ledger/internal/cases"
	"timber-pest-remediation-ledger/internal/httpapi"
	"timber-pest-remediation-ledger/internal/store"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return fmt.Errorf("配置错误: %w", err)
	}
	if cfg.SelfTest {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.SelfTestTimeout)
		defer cancel()
		return runSelfTest(ctx, cfg.Address)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DataPath), 0750); err != nil {
		return fmt.Errorf("创建数据目录: %w", err)
	}
	repository, err := store.Open(context.Background(), cfg.DataPath)
	if err != nil {
		return err
	}
	defer repository.Close()
	service := cases.NewService(repository)
	defer service.Close()
	server := httpapi.NewHTTPServer(cfg.Address, httpapi.New(service).Handler())
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.Address, err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	fmt.Printf("木构件虫害处置质量治理服务监听 %s\n", cfg.Address)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case sig := <-signals:
		fmt.Printf("收到 %s，开始关闭服务\n", sig)
	case serveErr := <-errCh:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("HTTP 服务退出: %w", serveErr)
		}
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("关闭 HTTP 服务: %w", err)
	}
	serveErr := <-errCh
	if !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	return nil
}
