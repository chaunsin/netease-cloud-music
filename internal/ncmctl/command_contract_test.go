// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/natefinch/lumberjack.v2"

	client "github.com/chaunsin/netease-cloud-music/api"
	cookiepkg "github.com/chaunsin/netease-cloud-music/pkg/cookie"
	projectlog "github.com/chaunsin/netease-cloud-music/pkg/log"
)

type fakeScheduledCommand struct {
	cmd *cobra.Command
}

func (f fakeScheduledCommand) Command() *cobra.Command {
	return f.cmd
}

func (fakeScheduledCommand) validate() error {
	return nil
}

func commandByPath(t *testing.T, path []string) *cobra.Command {
	t.Helper()

	root := New().cmd

	if len(path) == 0 {
		return root
	}

	command, _, err := root.Find(path)
	require.NoError(t, err)
	require.Equal(t, "ncmctl "+strings.Join(path, " "), command.CommandPath())

	return command
}

func TestCommandHelpContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path            []string
		use             string
		longContains    string
		exampleContains string
	}{
		{use: "ncmctl", longContains: "side effects", exampleContains: "ncmctl login qrcode"},
		{path: []string{"login"}, use: "login", longContains: "persists cookies", exampleContains: "ncmctl login phone"},
		{path: []string{"login", "phone"}, use: "phone <number>", longContains: "sends an SMS", exampleContains: "--password"},
		{path: []string{"login", "qrcode"}, use: "qrcode", longContains: "qrcode.png", exampleContains: "--dir"},
		{path: []string{"login", "cookie"}, use: "cookie [cookie]", longContains: "MUSIC_U", exampleContains: "--file"},
		{path: []string{"login", "cookiecloud"}, use: "cookiecloud", longContains: "--uuid and --password are required", exampleContains: "-s http"},
		{path: []string{"logout"}, use: "logout", longContains: "cookie.json", exampleContains: "ncmctl logout"},
		{path: []string{"task"}, use: "task", longContains: "all three jobs", exampleContains: "--sign --scrobble"},
		{path: []string{"sign"}, use: "sign", longContains: "Login is required", exampleContains: "--automatic"},
		{path: []string{"partner"}, use: "partner", longContains: "changes account state", exampleContains: "--star"},
		{path: []string{"scrobble"}, use: "scrobble", longContains: "account-restriction risk", exampleContains: "--num"},
		{path: []string{"download"}, use: "download <id-or-url> [id-or-url...]", longContains: "MD5 verification", exampleContains: "--level hires"},
		{path: []string{"cloud"}, use: "cloud <file-or-directory>", longContains: "500 MB", exampleContains: "--minsize"},
		{path: []string{"ncm"}, use: "ncm <input> [input...]", longContains: "Every positional argument", exampleContains: "--output"},
		{path: []string{"crypto"}, use: "crypto", longContains: "decryption currently supports EAPI only", exampleContains: "crypto encrypt"},
		{path: []string{"crypto", "encrypt"}, use: "encrypt <json-or-file>", longContains: "EAPI also requires --url", exampleContains: "request.json"},
		{path: []string{"crypto", "decrypt"}, use: "decrypt <ciphertext-or-har>", longContains: "Direct WEAPI", exampleContains: "--kind eapi"},
		{path: []string{"curl"}, use: "curl [method]", longContains: "not the system curl", exampleContains: "GetUserInfo"},
		{path: []string{"proxy"}, use: "proxy", longContains: "never modifies a trust store", exampleContains: "--ca-cert"},
	}

	for _, tt := range tests {
		name := strings.Join(tt.path, " ")

		if name == "" {
			name = "root"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			command := commandByPath(t, tt.path)

			assert.Equal(t, tt.use, command.Use)
			assert.NotEmpty(t, command.Short)
			assert.Contains(t, command.Long, tt.longContains)
			assert.Contains(t, command.Example, tt.exampleContains)
		})
	}
}

func TestCommandPositionalArgumentContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path    []string
		valid   [][]string
		invalid [][]string
	}{
		{path: []string{"login", "phone"}, valid: [][]string{{"18800008888"}}, invalid: [][]string{{}, {"one", "two"}}},
		{path: []string{"login", "cookie"}, valid: [][]string{{}, {"MUSIC_U=..."}}, invalid: [][]string{{"one", "two"}}},
		{path: []string{"login", "qrcode"}, valid: [][]string{{}}, invalid: [][]string{{"extra"}}},
		{path: []string{"login", "cookiecloud"}, valid: [][]string{{}}, invalid: [][]string{{"extra"}}},
		{path: []string{"logout"}, valid: [][]string{{}}, invalid: [][]string{{"extra"}}},
		{path: []string{"task"}, valid: [][]string{{}}, invalid: [][]string{{"sign"}}},
		{path: []string{"sign"}, valid: [][]string{{}}, invalid: [][]string{{"extra"}}},
		{path: []string{"partner"}, valid: [][]string{{}}, invalid: [][]string{{"extra"}}},
		{path: []string{"scrobble"}, valid: [][]string{{}}, invalid: [][]string{{"extra"}}},
		{path: []string{"download"}, valid: [][]string{{"1"}, {"1", "2"}}, invalid: [][]string{{}}},
		{path: []string{"cloud"}, valid: [][]string{{"music.mp3"}}, invalid: [][]string{{}, {"one", "two"}}},
		{path: []string{"ncm"}, valid: [][]string{{"one.ncm"}, {"one.ncm", "two.ncm"}}, invalid: [][]string{{}}},
		{path: []string{"crypto", "encrypt"}, valid: [][]string{{`{"key":"value"}`}}, invalid: [][]string{{}, {"one", "two"}}},
		{path: []string{"crypto", "decrypt"}, valid: [][]string{{"ciphertext"}}, invalid: [][]string{{}, {"one", "two"}}},
		{path: []string{"curl"}, valid: [][]string{{}, {"GetUserInfo"}}, invalid: [][]string{{"one", "two"}}},
		{path: []string{"proxy"}, valid: [][]string{{}}, invalid: [][]string{{"extra"}}},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.path, " "), func(t *testing.T) {
			t.Parallel()
			command := commandByPath(t, tt.path)

			for _, args := range tt.valid {
				require.NoError(t, command.Args(command, args), "valid args: %q", args)
			}

			for _, args := range tt.invalid {
				assert.Error(t, command.Args(command, args), "invalid args: %q", args)
			}
		})
	}
}

func TestCommandFlagDescriptionsExplainConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     []string
		flag     string
		contains string
	}{
		{flag: "debug", contains: "redacted"},
		{flag: "home", contains: "substituted for ${HOME}"},
		{flag: "config", contains: "not auto-discovered"},
		{path: []string{"login", "phone"}, flag: "password", contains: "process lists"},
		{path: []string{"login", "cookie"}, flag: "format", contains: "auto-detect"},
		{path: []string{"login", "cookiecloud"}, flag: "password", contains: "required"},
		{path: []string{"task"}, flag: "runAll", contains: "no task selectors"},
		{path: []string{"task"}, flag: "sign.automatic", contains: "account risk"},
		{path: []string{"partner"}, flag: "num", contains: "0 to 15"},
		{path: []string{"scrobble"}, flag: "num", contains: "1-300"},
		{path: []string{"download"}, flag: "parallel", contains: "1-20"},
		{path: []string{"download"}, flag: "tag", contains: "not implemented"},
		{path: []string{"cloud"}, flag: "parallel", contains: "1-10"},
		{path: []string{"ncm"}, flag: "tag", contains: "written by default"},
		{path: []string{"crypto"}, flag: "kind", contains: "decrypt supports eapi only"},
		{path: []string{"crypto", "encrypt"}, flag: "url", contains: "required"},
		{path: []string{"crypto", "decrypt"}, flag: "encode", contains: "string, hex, or base64"},
		{path: []string{"curl"}, flag: "method", contains: "not an HTTP verb"},
		{path: []string{"proxy"}, flag: "max-body", contains: "forwarding is unaffected"},
	}

	for _, tt := range tests {
		name := strings.Join(tt.path, " ")

		if name == "" {
			name = "root"
		}

		t.Run(name+" --"+tt.flag, func(t *testing.T) {
			t.Parallel()

			command := commandByPath(t, tt.path)
			flag := command.Flag(tt.flag)
			require.NotNil(t, flag)
			assert.Contains(t, flag.Usage, tt.contains)
		})
	}
}

func TestPartnerPropagatesCommandErrors(t *testing.T) {
	t.Parallel()

	command := NewPartner(&Root{}, nil).Command()
	assert.Nil(t, command.Run)
	require.NotNil(t, command.RunE)
}

func TestScheduledCommandClearsProcessArguments(t *testing.T) {
	logger := projectlog.New(&projectlog.Config{
		Level:  "info",
		Rotate: lumberjack.Logger{Filename: filepath.Join(t.TempDir(), "test.log")},
	})
	previous := projectlog.Default
	projectlog.Default = logger

	t.Cleanup(func() {
		projectlog.Default = previous
		_ = logger.Close()
	})

	executed := false
	command := &cobra.Command{
		Use:  "scheduled",
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			executed = true
			return nil
		},
	}
	task := &Task{cmd: &cobra.Command{}}

	err := task.registerScheduledCommand(
		context.Background(),
		cron.New(),
		"scheduled",
		"0 0 * * *",
		"schedule error",
		fakeScheduledCommand{cmd: command},
	)
	require.NoError(t, err)
	require.NoError(t, command.ExecuteContext(context.Background()))
	assert.True(t, executed)
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
	assert.Contains(t, command.Long, "Cookie-based login supports the following three content modes")
	assert.Contains(t, command.Long, "# Netscape HTTP Cookie File")
	assert.Contains(t, command.Long, "https://curl.se/rfc/cookie_spec.html")
	assert.Contains(t, command.Long, `"expirationDate"`)
	assert.Contains(t, command.Long, "not reusable credentials")
	assert.Contains(t, command.Long, "MUSIC_U\t<redacted>")
	assert.Contains(t, command.Long, `"name": "MUSIC_U"`)
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
