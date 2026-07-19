// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	client "github.com/chaunsin/netease-cloud-music/api"
	cookiepkg "github.com/chaunsin/netease-cloud-music/pkg/cookie"
)

func TestPartnerPropagatesCommandErrors(t *testing.T) {
	t.Parallel()

	command := NewPartner(&Root{}, nil).Command()
	assert.Nil(t, command.Run)
	require.NotNil(t, command.RunE)
}

func TestCloudRejectsZeroParallelism(t *testing.T) {
	t.Parallel()

	command := &Cloud{
		cmd:  &cobra.Command{},
		opts: CloudOpts{Parallel: 0},
	}
	err := command.execute(context.Background(), nil)
	require.EqualError(t, err, "parallel must be between 1 and 10")
}

func TestDownloadTagFlagDocumentsCompatibilityState(t *testing.T) {
	t.Parallel()

	flag := NewDownload(&Root{}, nil).Command().Flag("tag")
	require.NotNil(t, flag)
	assert.Contains(t, flag.Usage, "not implemented")
}

func TestCookieHelpUsesAcceptedNetscapeSpelling(t *testing.T) {
	t.Parallel()

	command := cookie(&Login{}, nil)
	assert.Contains(t, command.Example, "--format netscape")
	assert.NotContains(t, command.Example, "netscaple")
}

func TestValidateCurlKind(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"api", "eapi", "linux", "weapi"} {
		require.NoError(t, validateCurlKind(kind), kind)
	}

	require.EqualError(t, validateCurlKind("typo"), `unsupported API kind "typo"`)
}

func TestCloseAndRemoveDefaultCookie(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	cookiePath := filepath.Join(home, ".ncmctl", "cookie.json")
	cli, err := client.NewClient(&client.Config{
		Cookie: cookiepkg.Config{Filepath: cookiePath},
	}, nil)
	require.NoError(t, err)

	musicURL, err := url.Parse("https://music.163.com")
	require.NoError(t, err)
	cli.SetCookies(musicURL, []*http.Cookie{{Name: "MUSIC_U", Value: "token"}})
	require.FileExists(t, cookiePath)

	require.NoError(t, closeAndRemoveDefaultCookie(context.Background(), cli, home))
	assert.NoFileExists(t, cookiePath)

	// The deferred second close in logout must remain idempotent and not recreate the file.
	require.NoError(t, cli.Close(context.Background()))
	assert.NoFileExists(t, cookiePath)
}
