package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/phsym/console-slog"
	"github.com/walnuts1018/shutdown-manager/config"
	"github.com/walnuts1018/shutdown-manager/tracer"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt, os.Kill)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger := createLogger(cfg.LogLevel, cfg.LogType)
	slog.SetDefault(logger)

	closeTracer, err := tracer.NewTracerProvider(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create tracer provider", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer closeTracer()

	e := newRouter(cfg)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: e,
	}

	go func() {
		slog.Info("Server is running", slog.String("port", cfg.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.ErrorContext(ctx, "Failed to run server", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	stop()
	slog.Info("Received shutdown signal, shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.ErrorContext(ctx, "Failed to shutdown server", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func createLogger(logLevel slog.Level, logType config.LogType) *slog.Logger {
	var hander slog.Handler
	switch logType {
	case config.LogTypeText:
		hander = console.NewHandler(os.Stdout, &console.HandlerOptions{
			Level:     logLevel,
			AddSource: logLevel == slog.LevelDebug,
		})
	case config.LogTypeJSON:
		hander = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     logLevel,
			AddSource: logLevel == slog.LevelDebug,
		})
	}

	return slog.New(hander)
}

func newRouter() *echo.Echo {
	e := echo.New()

	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Skipper: skipper,
	}))
	e.Use(middleware.Recover())
	e.Use(otelecho.Middleware(tracer.ServiceName, otelecho.WithSkipper(
		skipper,
	)))

	e.GET("/livez", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}).Name = "health.liveness"
	e.GET("/readyz", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}).Name = "health.readiness"

	e.POST("/shutdown", func(c echo.Context) error {
		if err := shutdown(); err != nil {
			slog.Error("Failed to shutdown", slog.String("error", err.Error()))
			return c.String(http.StatusInternalServerError, "failed to shutdown")
		}
		return c.String(http.StatusOK, "shutdown command executed")
	}).Name = "shutdown"

	return e
}

func skipper(c echo.Context) bool {
	return c.Path() == "/livez" || c.Path() == "/readyz"
}

func shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", "poweroff", "-i")
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute shutdown command: %w, stdout: %s, stderr: %s", err, stdout.String(), stderr.String())
	}
	slog.Info("Shutdown command executed successfully", slog.String("output", stdout.String()))
	return nil
}
