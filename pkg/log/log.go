// MIT License
//
// Copyright (c) 2024 chaunsin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

package log

import (
	"context"
	"fmt"
	"io"
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

type Logger struct {
	l     *slog.Logger
	level *slog.LevelVar
}

func New(cfg *Config) *Logger {
	if cfg == nil {
		cfg = &defaultConfig
	}

	var level slog.LevelVar
	switch cfg.Level {
	case "debug":
		level.Set(slog.LevelDebug)
	case "info":
		level.Set(slog.LevelInfo)
	case "level":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelDebug)
	}

	var opts = slog.HandlerOptions{
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
		fallthrough
	default:
		h = slog.NewTextHandler(io.MultiWriter(w...), &opts)
	}
	h = h.WithAttrs([]slog.Attr{slog.String("app", cfg.App)})

	l := Logger{
		l:     slog.New(h),
		level: &level,
	}
	return &l
}

func (l Logger) Logger() *slog.Logger {
	return l.l
}

func log(h slog.Handler, lv slog.Level, msg string, args ...any) {
	// 需要检查是否满足日志级别？
	if !h.Enabled(context.Background(), lv) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // skip [Callers, Info]
	r := slog.NewRecord(time.Now(), lv, msg, pcs[0])
	r.Add(args...)
	if err := h.Handle(ctx, r); err != nil {
		fmt.Printf("[log] handle err:%s\n", err)
	}
}

func Debug(format string, args ...any) {
	log(Default.l.Handler(), slog.LevelDebug, format, args...)
}

func Info(format string, args ...any) {
	log(Default.l.Handler(), slog.LevelInfo, fmt.Sprintf(format, args...))
}

func Warn(format string, args ...any) {
	log(Default.l.Handler(), slog.LevelWarn, fmt.Sprintf(format, args...))
}

func Error(format string, args ...any) {
	log(Default.l.Handler(), slog.LevelError, fmt.Sprintf(format, args...))
}

func Fatal(format string, args ...any) {
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
