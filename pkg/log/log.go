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
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Default       *Logger
	ctx           = context.Background()
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

	l := Logger{
		cfg:   cfg,
		l:     slog.New(h),
		level: &level,
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

func log(h slog.Handler, lv slog.Level, msg string, args ...any) {
	// 需要检查是否满足日志级别？
	if !h.Enabled(ctx, lv) {
		return
	}

	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // skip [Callers, Info]
	r := slog.NewRecord(time.Now(), lv, msg, pcs[0])
	r.Add(args...)

	if err := h.Handle(ctx, r); err != nil {
		stdlog.Printf("[log] handler error: %v", err)
	}
}

func Debugf(format string, args ...any) {
	log(Default.l.Handler(), slog.LevelDebug, fmt.Sprintf(format, args...))
}

func Infof(format string, args ...any) {
	log(Default.l.Handler(), slog.LevelInfo, fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...any) {
	log(Default.l.Handler(), slog.LevelWarn, fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...any) {
	log(Default.l.Handler(), slog.LevelError, fmt.Sprintf(format, args...))
}

func Fatalf(format string, args ...any) {
	log(Default.l.Handler(), slog.LevelError, fmt.Sprintf(format, args...))
	os.Exit(1)
}

func DebugW(msg string, args ...any) {
	log(Default.l.Handler(), slog.LevelDebug, msg, args...)
}

func InfoW(msg string, args ...any) {
	log(Default.l.Handler(), slog.LevelInfo, msg, args...)
}

func WarnW(msg string, args ...any) {
	log(Default.l.Handler(), slog.LevelWarn, msg, args...)
}

func ErrorW(msg string, args ...any) {
	log(Default.l.Handler(), slog.LevelError, msg, args...)
}

func FatalW(msg string, args ...any) {
	log(Default.l.Handler(), slog.LevelError, msg, args...)
	os.Exit(1)
}
