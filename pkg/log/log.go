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
	"fmt"
	"io"
	"log/slog"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Default       *Logger
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

func Debug(msg string, args ...any) {
	Default.l.Debug(fmt.Sprintf(msg, args...))
}

func Info(msg string, args ...any) {
	Default.l.Info(fmt.Sprintf(msg, args...))
}

func Warn(msg string, args ...any) {
	Default.l.Warn(fmt.Sprintf(msg, args...))
}

func Error(msg string, args ...any) {
	Default.l.Error(fmt.Sprintf(msg, args...))
}

func Fatal(msg string, args ...any) {
	Default.l.Error(fmt.Sprintf(msg, args...))
	os.Exit(1)
}

func DebugW(msg string, args ...any) {
	Default.l.Debug(msg, args...)
}

func InfoW(msg string, args ...any) {
	Default.l.Info(msg, args...)
}

func WarnW(msg string, args ...any) {
	Default.l.Warn(msg, args...)
}

func ErrorW(msg string, args ...any) {
	Default.l.Error(msg, args...)
}

func FatalW(msg string, args ...any) {
	Default.l.Error(msg, args...)
	os.Exit(1)
}
