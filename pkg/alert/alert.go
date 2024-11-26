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

func New(module Module, cfg *Config) (a Alert, err error) {
	switch module {
	case ModuleMail:
		a, err = mail.New(cfg.Mail)
	case ModuleHTTP:
		a, err = http.New(cfg.HTTP)
	default:
		return nil, errors.New("invalid module")
	}
	return
}
