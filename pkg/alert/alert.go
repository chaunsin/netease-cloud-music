// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package alert

import (
	"context"
	"errors"

	"github.com/chaunsin/netease-cloud-music/pkg/alert/http"
	"github.com/chaunsin/netease-cloud-music/pkg/alert/mail"
)

type Config struct {
	Module Module       `json:"module" yaml:"module"`
	Mail   *mail.Config `json:"mail" yaml:"mail"`
	HTTP   *http.Config `json:"http" yaml:"http"`
}

type Module string

const (
	ModuleMail Module = "mail"
	ModuleHTTP Module = "http"
	ModuleVX   Module = "vx"
)

type Alert interface {
	Send(ctx context.Context, content string) error
	Close(ctx context.Context) error
}

func New(module Module, cfg *Config) (Alert, error) {
	switch module {
	case ModuleMail:
		return mail.New(cfg.Mail)
	case ModuleHTTP:
		return http.New(cfg.HTTP)
	default:
		return nil, errors.New("invalid module")
	}
}
