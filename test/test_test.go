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
			Debug:   false,
			Timeout: 0,
			Retry:   0,
		},
	}
	cli = api.New(&cfg)
	os.Exit(t.Run())
}
