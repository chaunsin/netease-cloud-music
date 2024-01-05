package test

import (
	"context"
	"os"
	"testing"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/config"
)

var (
	cli *api.Client
	ctx = context.TODO()
)

func TestMain(t *testing.M) {
	cfg := config.Config{
		Network: config.Network{
			Url:     "",
			Debug:   false,
			Timeout: 0,
			Retry:   0,
		},
		Crypto: config.Crypto{},
	}
	cli = api.New(&cfg)
	os.Exit(t.Run())
}
