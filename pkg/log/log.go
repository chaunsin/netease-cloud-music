// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package log

import (
	"context"
	"fmt"
	"io"
	stdlog "log"
	"log/slog"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Default       = New(&defaultConfig)
	hostname, _   = os.Hostname()
	defaultConfig = Config{
		App:    hostname,
		Format: "text",
		Level:  "info",
		Stdout: true,
		Rotate: lumberjack.Logger{
			Filename:   "./log/info.log",
			MaxSize:    100,
			MaxAge:     7,
			MaxBackups: 10,
			LocalTime:  true,
			Compress:   true,
		},
	}
)

type Config struct {
	App    string            `json:"app,omitempty" yaml:"app"`
	Format string            `json:"format,omitempty" yaml:"format"` // text(default) json
	Level  string            `json:"level,omitempty" yaml:"level"`   // debug(default) < info < warn < error
	Stdout bool              `json:"stdout,omitempty" yaml:"stdout"`
	Rotate lumberjack.Logger `json:"rotate" yaml:"rotate"`
}

func (c *Config) Validate() error {
	switch c.Format {
	case "", "text", "json":
	default:
		return fmt.Errorf("unsupported log format %q", c.Format)
	}

	switch c.Level {
	case "", "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("unsupported log level %q", c.Level)
	}
	return nil
}

type Logger struct {
	cfg   *Config
	l     *slog.Logger
	level *slog.LevelVar
	skip  *atomic.Int32
}

func New(cfg *Config) *Logger {
	if cfg == nil {
		cfg = &defaultConfig
	}

	if err := cfg.Validate(); err != nil {
		panic(fmt.Sprintf("config validate: %s", err))
	}

	var level slog.LevelVar

	switch cfg.Level {
	case "debug":
		level.Set(slog.LevelDebug)
	case "info":
		level.Set(slog.LevelInfo)
	case "warn":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelDebug)
	}

	opts := slog.HandlerOptions{
		AddSource:   true,
		Level:       &level,
		ReplaceAttr: nil,
	}

	var w []io.Writer
	if cfg.Stdout {
		w = append(w, os.Stderr)
	}

	w = append(w, &cfg.Rotate)

	var h slog.Handler

	switch cfg.Format {
	case "json":
		h = slog.NewJSONHandler(io.MultiWriter(w...), &opts)
	case "text":
		h = slog.NewTextHandler(io.MultiWriter(w...), &opts)
	default:
		h = slog.NewTextHandler(io.MultiWriter(w...), &opts)
	}

	h = h.WithAttrs([]slog.Attr{slog.String("app", cfg.App)})
	skip := new(atomic.Int32)
	skip.Store(3)

	l := Logger{
		cfg:   cfg,
		l:     slog.New(h),
		level: &level,
		skip:  skip, // default 3
	}
	return &l
}

func (l *Logger) Close() error {
	if l == nil || l.cfg == nil {
		return nil
	}
	return l.cfg.Rotate.Close()
}

func (l *Logger) Logger() *slog.Logger {
	return l.l
}

func (l *Logger) SetLevel(level slog.Level) {
	l.level.Set(level)
}

func (l *Logger) SetRuntimeSkip(n int32) {
	if n < 0 {
		return
	}

	l.skip.Store(n)
}

func (l *Logger) Debug(msg string, args ...any) {
	l.log(context.Background(), slog.LevelDebug, msg, args...)
}

func (l *Logger) Debugf(format string, args ...any) {
	l.log(context.Background(), slog.LevelDebug, fmt.Sprintf(format, args...))
}

func (l *Logger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelDebug, msg, args...)
}

//nolint:goprintffuncname // Preserve DebugfContext for API compatibility.
func (l *Logger) DebugfContext(ctx context.Context, format string, args ...any) {
	l.log(ctx, slog.LevelDebug, fmt.Sprintf(format, args...))
}

func (l *Logger) Info(msg string, args ...any) {
	l.log(context.Background(), slog.LevelInfo, msg, args...)
}

func (l *Logger) Infof(format string, args ...any) {
	l.log(context.Background(), slog.LevelInfo, fmt.Sprintf(format, args...))
}

func (l *Logger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelInfo, msg, args...)
}

//nolint:goprintffuncname // Match DebugfContext's API naming convention.
func (l *Logger) InfofContext(ctx context.Context, format string, args ...any) {
	l.log(ctx, slog.LevelInfo, fmt.Sprintf(format, args...))
}

func (l *Logger) Warn(msg string, args ...any) {
	l.log(context.Background(), slog.LevelWarn, msg, args...)
}

func (l *Logger) Warnf(format string, args ...any) {
	l.log(context.Background(), slog.LevelWarn, fmt.Sprintf(format, args...))
}

func (l *Logger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelWarn, msg, args...)
}

//nolint:goprintffuncname // Match DebugfContext's API naming convention.
func (l *Logger) WarnfContext(ctx context.Context, format string, args ...any) {
	l.log(ctx, slog.LevelWarn, fmt.Sprintf(format, args...))
}

func (l *Logger) Error(msg string, args ...any) {
	l.log(context.Background(), slog.LevelError, msg, args...)
}

func (l *Logger) Errorf(format string, args ...any) {
	l.log(context.Background(), slog.LevelError, fmt.Sprintf(format, args...))
}

func (l *Logger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelError, msg, args...)
}

//nolint:goprintffuncname // Match DebugfContext's API naming convention.
func (l *Logger) ErrorfContext(ctx context.Context, format string, args ...any) {
	l.log(ctx, slog.LevelError, fmt.Sprintf(format, args...))
}

func (l *Logger) log(ctx context.Context, level slog.Level, msg string, args ...any) {
	if l == nil || l.l == nil {
		return
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if !l.l.Enabled(ctx, level) {
		return
	}

	var pcs [1]uintptr
	runtime.Callers(int(l.skip.Load()), pcs[:])
	record := slog.NewRecord(time.Now(), level, msg, pcs[0])
	record.Add(args...)

	if err := l.l.Handler().Handle(ctx, record); err != nil {
		stdlog.Printf("[log] handler error: %v", err)
	}
}

func Debugf(format string, args ...any) {
	Default.log(context.Background(), slog.LevelDebug, fmt.Sprintf(format, args...))
}

func Infof(format string, args ...any) {
	Default.log(context.Background(), slog.LevelInfo, fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...any) {
	Default.log(context.Background(), slog.LevelWarn, fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...any) {
	Default.log(context.Background(), slog.LevelError, fmt.Sprintf(format, args...))
}

func Fatalf(format string, args ...any) {
	Default.log(context.Background(), slog.LevelError, fmt.Sprintf(format, args...))
	os.Exit(1)
}

func Debug(msg string, args ...any) {
	Default.log(context.Background(), slog.LevelDebug, msg, args...)
}

func Info(msg string, args ...any) {
	Default.log(context.Background(), slog.LevelInfo, msg, args...)
}

func Warn(msg string, args ...any) {
	Default.log(context.Background(), slog.LevelWarn, msg, args...)
}

func Error(msg string, args ...any) {
	Default.log(context.Background(), slog.LevelError, msg, args...)
}

func Fatal(msg string, args ...any) {
	Default.log(context.Background(), slog.LevelError, msg, args...)
	os.Exit(1)
}
