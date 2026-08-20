// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testLatestPath   = "/https://github.com/chaunsin/netease-cloud-music/releases/latest"
	testTagPath      = "/https://github.com/chaunsin/netease-cloud-music/releases/tag/"
	testDownloadPath = "/https://github.com/chaunsin/netease-cloud-music/releases/download/"
)

func TestCompareSemver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		left   string
		right  string
		want   int
		errMsg string
	}{
		{name: "equal with build metadata", left: "v1.2.3", right: "1.2.3+build.7", want: 0},
		{name: "major wins", left: "2.0.0", right: "1.99.99", want: 1},
		{name: "prerelease is lower", left: "1.2.3", right: "1.2.3-rc.1", want: 1},
		{name: "numeric prerelease compare", left: "1.2.3-alpha.2", right: "1.2.3-alpha.10", want: -1},
		{name: "numeric before alphanumeric", left: "1.2.3-1", right: "1.2.3-alpha", want: -1},
		{name: "shorter prerelease list is lower", left: "1.2.3-alpha", right: "1.2.3-alpha.1", want: -1},
		{name: "identical", left: "1.2.3", right: "1.2.3", want: 0},
		{name: "identical prerelease", left: "v1.2.3-rc.1", right: "1.2.3-rc.1", want: 0},
		{name: "build metadata ignored", left: "1.2.3-rc.1+build.5", right: "1.2.3-rc.1", want: 0},
		{name: "big numbers", left: "999999999999999999999.0.0", right: "1.0.0", want: 1},
		{name: "leading zeros invalid", left: "1.02.3", right: "1.2.3", errMsg: "invalid SemVer"},
		{name: "missing minor", left: "1.2", right: "1.2.3", errMsg: "invalid SemVer"},
		{name: "not a version", left: "abc", right: "1.2.3", errMsg: "invalid SemVer"},
		{name: "empty", left: "", right: "1.2.3", errMsg: "invalid SemVer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			left, err := parseSemver(tt.left)
			if tt.errMsg != "" {
				require.ErrorContains(t, err, tt.errMsg)
				return
			}

			require.NoError(t, err)

			right, err := parseSemver(tt.right)
			require.NoError(t, err)
			require.Equal(t, tt.want, left.Compare(right))
		})
	}
}

func TestAssetName(t *testing.T) {
	t.Parallel()

	osNames := map[string]string{
		"linux": "Linux", "darwin": "Darwin", "windows": "Windows",
		"freebsd": "Freebsd", "openbsd": "Openbsd", "netbsd": "Netbsd",
	}
	archNames := map[string]string{
		"amd64": "x86_64", "386": "i386", "arm64": "arm64", "s390x": "s390x",
		"ppc64": "ppc64", "ppc64le": "ppc64le", "riscv64": "riscv64",
		"mips": "mips", "mipsle": "mipsle", "mips64": "mips64", "mips64le": "mips64le",
		"loong64": "loong64",
	}

	for goos, osName := range osNames {
		for goarch, archName := range archNames {
			want := "ncmctl_" + osName + "_" + archName + ".tar.gz"
			if goos == "windows" {
				want = "ncmctl_" + osName + "_" + archName + ".zip"
			}

			got, err := assetName(goos, goarch, "")
			require.NoError(t, err)
			require.Equal(t, want, got)
		}
	}

	got, err := assetName("linux", "arm", "6")
	require.NoError(t, err)
	require.Equal(t, "ncmctl_Linux_armv6.tar.gz", got)

	_, err = assetName("linux", "arm", "7")
	require.ErrorContains(t, err, "unsupported GOARM: 7")
	_, err = assetName("plan9", "amd64", "")
	require.ErrorContains(t, err, "unsupported GOOS: plan9")
	_, err = assetName("linux", "wasm", "")
	require.ErrorContains(t, err, "unsupported GOARCH: wasm")
}

func TestValidateProxy(t *testing.T) {
	t.Parallel()

	invalid := []string{
		"http://insecure.example/",
		"https:///missing-host",
		"https://:",
		"https://@/",
		"https://example.com:0/",
		"https://example.com:65536/",
		"https://example.com:not-a-port/",
	}
	for _, prefix := range invalid {
		_, err := validateProxy(prefix)
		require.Error(t, err, "prefix %q", prefix)
	}

	got, err := validateProxy("https://proxy.example/private-token/")
	require.NoError(t, err)
	require.Equal(t, "https://proxy.example/private-token/", got)

	got, err = validateProxy("https://proxy.example")
	require.NoError(t, err)
	require.Equal(t, "https://proxy.example/", got)

	_, err = validateProxy("https://user:secret@proxy.example/")
	require.Error(t, err)
}

func TestRouteName(t *testing.T) {
	t.Parallel()

	got := routeName("https://user:secret@proxy.example:8443/private/token?access=hidden")
	require.Equal(t, "HTTPS proxy proxy.example:8443", got)
	require.Equal(t, "GitHub", routeName(""))
	require.Equal(t, "HTTPS proxy proxy.example", routeName("https://proxy.example/"))
	require.Equal(t, "configured HTTPS proxy", routeName("https://bad host/"))
}

func TestVersionFromReleaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		releaseURL string
		prefix     string
		want       string
		errMsg     string
	}{
		{name: "direct", releaseURL: githubRepoURL + "/releases/tag/v1.2.3", want: "v1.2.3"},
		{name: "proxy prefix", releaseURL: "https://proxy.example/" + githubRepoURL + "/releases/tag/v1.2.3-alpha.4", prefix: "https://proxy.example/", want: "v1.2.3-alpha.4"},
		{name: "bounce back to github", releaseURL: githubRepoURL + "/releases/tag/v1.2.3", prefix: "https://proxy.example/", want: "v1.2.3"},
		{name: "foreign repository", releaseURL: "https://github.com/other/releases/tag/v1.2.3", errMsg: "unexpected release URL"},
		{
			name:       "foreign repository via proxy",
			releaseURL: "https://proxy.example/" + "https://github.com/other/releases/tag/v1.2.3",
			prefix:     "https://proxy.example/",
			errMsg:     "unexpected release URL",
		},
		{name: "extra path", releaseURL: githubRepoURL + "/releases/tag/v1.2.3/notes", errMsg: "invalid release tag"},
		{name: "query", releaseURL: githubRepoURL + "/releases/tag/v1.2.3?x=1", errMsg: "invalid release tag"},
		{name: "tag without v", releaseURL: githubRepoURL + "/releases/tag/1.2.3", errMsg: "invalid release tag"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := versionFromReleaseURL(tt.releaseURL, tt.prefix)
			if tt.errMsg != "" {
				require.ErrorContains(t, err, tt.errMsg)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// The following tests exercise the Windows branch of replaceExecutable on any
// platform by temporarily overriding the package-level windowsReplace hook,
// mirroring the removeFile injection used below. They mutate package state and
// therefore must not run in parallel with other tests.
func TestReplaceExecutableWindows(t *testing.T) {
	u, _, _ := newUpdateHarness(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "ncmctl")
	staged := filepath.Join(dir, ".staged")

	require.NoError(t, os.WriteFile(target, []byte("old"), 0o600))
	require.NoError(t, os.WriteFile(staged, []byte("new"), 0o600))

	original := windowsReplace
	windowsReplace = true

	t.Cleanup(func() { windowsReplace = original })

	require.NoError(t, u.replaceExecutable(target, staged))

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "new", string(content))

	_, err = os.Stat(target + ".old")
	require.True(t, os.IsNotExist(err))
}

func TestReplaceExecutableWindowsRollback(t *testing.T) {
	u, _, _ := newUpdateHarness(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "ncmctl")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o600))
	staged := filepath.Join(filepath.Join(t.TempDir(), "missing"), "staged")

	original := windowsReplace
	windowsReplace = true

	t.Cleanup(func() { windowsReplace = original })

	err := u.replaceExecutable(target, staged)
	require.Error(t, err)

	content, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	require.Equal(t, "old", string(content))

	_, statErr := os.Stat(target + ".old")
	require.True(t, os.IsNotExist(statErr))
}

func TestReplaceExecutableWindowsMissingTarget(t *testing.T) {
	u, _, _ := newUpdateHarness(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "missing")
	staged := filepath.Join(dir, ".staged")
	require.NoError(t, os.WriteFile(staged, []byte("new"), 0o600))

	original := windowsReplace
	windowsReplace = true

	t.Cleanup(func() { windowsReplace = original })

	require.NoError(t, u.replaceExecutable(target, staged))

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "new", string(content))

	_, err = os.Stat(target + ".old")
	require.True(t, os.IsNotExist(err))
}

// TestReplaceExecutableWindowsOldRemoveWarning verifies that a failure to
// remove the superseded binary (the running process always locks the old
// executable on Windows) is downgraded to a warning while the replacement is
// treated as successful, leaving the leftover for the next update to cover.
// It mutates the package-level removeFile and windowsReplace hooks and
// therefore must not run in parallel with other tests.
func TestReplaceExecutableWindowsOldRemoveWarning(t *testing.T) {
	u, _, errOut := newUpdateHarness(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "ncmctl")
	staged := filepath.Join(dir, ".staged")

	require.NoError(t, os.WriteFile(target, []byte("old"), 0o600))
	require.NoError(t, os.WriteFile(staged, []byte("new"), 0o600))

	originalRemove := removeFile
	removeFile = func(string) error { return errors.New("simulated removal failure") }

	t.Cleanup(func() { removeFile = originalRemove })

	originalWindows := windowsReplace
	windowsReplace = true

	t.Cleanup(func() { windowsReplace = originalWindows })

	require.NoError(t, u.replaceExecutable(target, staged))

	content, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	require.Equal(t, "new", string(content))

	_, statErr := os.Stat(target + ".old")
	require.NoError(t, statErr)

	require.Contains(t, errOut.String(), "The previous binary could not be removed ("+target+".old)")
}

func TestDownloadFileSizeLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response func(w http.ResponseWriter)
	}{
		{
			name: "content length exceeded",
			response: func(w http.ResponseWriter) {
				w.Header().Set("Content-Length", "999999")
				_, _ = w.Write([]byte("x"))
			},
		},
		{
			name: "streamed body exceeded",
			response: func(w http.ResponseWriter) {
				_, _ = io.WriteString(w, strings.Repeat("x", 4096))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.response(w)
			}))
			t.Cleanup(server.Close)

			dest := filepath.Join(t.TempDir(), "out.part")
			err := (&Update{}).downloadFile(context.Background(), server.Client(), server.URL, dest, time.Second, 1024)
			require.Error(t, err)
			require.ErrorContains(t, err, "size limit")

			_, statErr := os.Stat(dest)
			require.True(t, os.IsNotExist(statErr))
		})
	}
}

func TestExtractBinarySizeLimit(t *testing.T) {
	t.Parallel()

	entries := []archiveEntry{{name: "ncmctl", mode: 0o755, content: bytes.Repeat([]byte("x"), 2048)}}

	tests := []struct {
		name       string
		archiveExt string
	}{
		{name: "tar.gz", archiveExt: "tar.gz"},
		{name: "zip", archiveExt: "zip"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			archivePath := filepath.Join(dir, "release."+tt.archiveExt)

			var data []byte
			if tt.archiveExt == "zip" {
				data = makeZip(t, entries)
			} else {
				data = makeTarGz(t, entries)
			}

			require.NoError(t, os.WriteFile(archivePath, data, 0o600))

			_, err := extractBinaryFromArchive(archivePath, dir, "ncmctl", 1024)
			require.ErrorIs(t, err, errEntryTooLarge)

			leftovers, globErr := filepath.Glob(filepath.Join(dir, ".ncmctl.update-*"))
			require.NoError(t, globErr)
			require.Empty(t, leftovers)
		})
	}
}

func TestBuildRoutes(t *testing.T) {
	t.Parallel()

	got, err := buildRoutes("", false)
	require.NoError(t, err)
	require.Equal(t, append(append([]string{}, defaultGithubProxies...), ""), got)

	got, err = buildRoutes("", true)
	require.NoError(t, err)
	require.Equal(t, []string{""}, got)

	got, err = buildRoutes("https://a.example/ https://b.example/", true)
	require.NoError(t, err)
	require.Equal(t, []string{"https://a.example/", "https://b.example/", ""}, got)

	_, err = buildRoutes("https://a.example http://b.example", true)
	require.ErrorContains(t, err, "invalid HTTPS proxy at position 2: scheme must be https")
}

func TestNewUpdateHTTPClientRejectsNonHTTPSRedirect(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.com/", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	_, err := newUpdateHTTPClient().Head(server.URL) //nolint:bodyclose // The client reports a redirect error before returning the response body.
	require.Error(t, err)
	require.ErrorContains(t, err, "redirect to a non-HTTPS URL")
}

func TestExtractBinaryFromArchive(t *testing.T) {
	t.Parallel()

	valid := []byte("#!/bin/sh\necho v1.2.3\n")

	tests := []struct {
		name       string
		entries    []archiveEntry
		archiveExt string
		errMsg     string
	}{
		{name: "valid tar.gz", entries: []archiveEntry{{name: "ncmctl", mode: 0o755, content: valid}}, archiveExt: "tar.gz"},
		{name: "traversal entry skipped", entries: []archiveEntry{{name: "../evil", mode: 0o755, content: []byte("x")}, {name: "ncmctl", mode: 0o755, content: valid}}, archiveExt: "tar.gz"},
		{name: "only traversal entry", entries: []archiveEntry{{name: "../evil", mode: 0o755, content: []byte("x")}}, archiveExt: "tar.gz", errMsg: "does not contain"},
		{name: "symlink entry", entries: []archiveEntry{{name: "ncmctl", mode: 0o755, symlink: "/usr/bin/ncmctl"}}, archiveExt: "tar.gz", errMsg: "does not contain"},
		{name: "empty archive", entries: nil, archiveExt: "tar.gz", errMsg: "does not contain"},
		{name: "valid zip", entries: []archiveEntry{{name: "ncmctl", mode: 0o755, content: valid}}, archiveExt: "zip"},
		{name: "directory entry zip", entries: []archiveEntry{{name: "ncmctl/", mode: 0o755, content: nil}}, archiveExt: "zip", errMsg: "does not contain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			archivePath := filepath.Join(t.TempDir(), "release."+tt.archiveExt)

			var data []byte
			if tt.archiveExt == "zip" {
				data = makeZip(t, tt.entries)
			} else {
				data = makeTarGz(t, tt.entries)
			}

			require.NoError(t, os.WriteFile(archivePath, data, 0o600))

			staged, err := extractBinaryFromArchive(archivePath, t.TempDir(), "ncmctl", archiveMaxSize)
			if tt.errMsg != "" {
				require.ErrorContains(t, err, tt.errMsg)
				return
			}

			require.NoError(t, err)

			defer func() { _ = os.Remove(staged) }()

			info, statErr := os.Stat(staged)
			require.NoError(t, statErr)
			require.Equal(t, fs.FileMode(0o755), info.Mode().Perm())

			content, readErr := os.ReadFile(staged)
			require.NoError(t, readErr)
			require.Equal(t, valid, content)
		})
	}
}

func TestBinaryVersion(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "ncmctl")
	writeVersionScript(t, path, "v1.2.3")

	got, err := binaryVersion(path)
	require.NoError(t, err)
	require.Equal(t, "v1.2.3", got)

	writeRawScript(t, path, "#!/usr/bin/env bash\necho ncmctl\n")
	_, err = binaryVersion(path)
	require.ErrorContains(t, err, "no Version line found")

	writeRawScript(t, path, "#!/usr/bin/env bash\necho 'Version: '\n")
	_, err = binaryVersion(path)
	require.ErrorContains(t, err, "no Version line found")
}

func TestCheckUpToDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		installed string
		latest    string
		proceed   bool
		stdoutHas string
		stderrHas string
	}{
		{name: "up to date", installed: "v1.2.3", latest: "v1.2.3", stdoutHas: "ncmctl is up-to-date (version: v1.2.3)."},
		{name: "newer installed", installed: "v2.0.0", latest: "v1.2.3", stdoutHas: "Installed version v2.0.0 is newer than GitHub Release v1.2.3; skipping downgrade."},
		{name: "older installed", installed: "v1.0.0", latest: "v1.2.3", proceed: true, stdoutHas: "Installed version: v1.0.0. A newer version (v1.2.3) is available."},
		{name: "non semver", installed: "master", latest: "v1.2.3", proceed: true, stderrHas: "Installed version master is not valid SemVer; reinstalling v1.2.3."},
		{name: "invalid latest", installed: "v1.0.0", latest: "master", proceed: true, stderrHas: "Installed version master is not valid SemVer; reinstalling master."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u, out, errOut := newUpdateHarness(t)
			target := filepath.Join(t.TempDir(), "ncmctl")
			writeVersionScript(t, target, tt.installed)

			proceed := u.checkUpToDate(&updateState{target: target, latest: tt.latest})

			require.Equal(t, tt.proceed, proceed)

			if tt.stdoutHas != "" {
				assert.Contains(t, out.String(), tt.stdoutHas)
			}

			if tt.stderrHas != "" {
				assert.Contains(t, errOut.String(), tt.stderrHas)
			}
		})
	}
}

func TestCheckUpToDateUnreadableBinary(t *testing.T) {
	t.Parallel()

	u, _, errOut := newUpdateHarness(t)
	target := filepath.Join(t.TempDir(), "missing")

	proceed := u.checkUpToDate(&updateState{target: target, latest: "v1.2.3"})

	require.True(t, proceed)
	require.Contains(t, errOut.String(), "Unable to read the installed version; reinstalling.")
}

func TestUpdateUpgrade(t *testing.T) {
	t.Parallel()

	s := newUpdateScenario(t, "v1.0.0")

	require.NoError(t, s.run(t))
	require.Equal(t, "v1.2.3", mustVersion(t, s.target))
	require.Contains(t, s.out.String(), "ncmctl installed successfully at "+s.resolvedTarget(t)+" (version: v1.2.3).")
	require.Contains(t, s.out.String(), "Restart ncmctl to use the new version.")

	head, checksum, archive := s.f.counts()
	require.Equal(t, 1, head)
	require.Equal(t, 1, checksum)
	require.Equal(t, 1, archive)

	_, err := os.Stat(filepath.Join(filepath.Dir(s.target), installLockName))
	require.True(t, os.IsNotExist(err))
}

func TestUpdateUpToDate(t *testing.T) {
	t.Parallel()

	s := newUpdateScenario(t, "v1.2.3")

	require.NoError(t, s.run(t))
	require.Contains(t, s.out.String(), "ncmctl is up-to-date (version: v1.2.3).")

	head, checksum, archive := s.f.counts()
	require.Equal(t, 1, head)
	require.Zero(t, checksum)
	require.Zero(t, archive)
}

func TestUpdateDowngradeSkipped(t *testing.T) {
	t.Parallel()

	s := newUpdateScenario(t, "v2.0.0")

	require.NoError(t, s.run(t))
	require.Contains(t, s.out.String(), "Installed version v2.0.0 is newer than GitHub Release v1.2.3; skipping downgrade.")
	require.Equal(t, "v2.0.0", mustVersion(t, s.target))

	_, checksum, archive := s.f.counts()
	require.Zero(t, checksum)
	require.Zero(t, archive)
}

func TestUpdateNonSemverLocal(t *testing.T) {
	t.Parallel()

	s := newUpdateScenario(t, "master")

	require.NoError(t, s.run(t))
	require.Contains(t, s.errOut.String(), "Installed version master is not valid SemVer; reinstalling v1.2.3.")
	require.Equal(t, "v1.2.3", mustVersion(t, s.target))
}

func TestUpdateProxyFallback(t *testing.T) {
	t.Parallel()

	asset, err := assetName(runtime.GOOS, runtime.GOARCH, buildGOARM())
	require.NoError(t, err)
	archive, checksum := makeReleaseArchive(t, "v1.2.3", binaryNameForHost())
	first := newFakeRelease(t, asset, archive, checksum)
	first.failHead = true
	second := newFakeRelease(t, asset, archive, checksum)

	u, out, errOut := newUpdateHarness(t)
	u.httpClient = first.server.Client()
	target := filepath.Join(t.TempDir(), "ncmctl")
	writeVersionScript(t, target, "v1.0.0")

	routes := []string{first.server.URL + "/", second.server.URL + "/"}

	err = u.run(context.Background(), &updateState{routes: routes, target: target})
	require.NoError(t, err)
	require.Equal(t, "v1.2.3", mustVersion(t, target))
	require.Contains(t, errOut.String(), "Failed to resolve the latest release tag via "+routeName(routes[0])+".")

	resolved, err := filepath.EvalSymlinks(target)
	require.NoError(t, err)
	require.Contains(t, out.String(), "ncmctl installed successfully at "+resolved+" (version: v1.2.3).")

	head, checksumReqs, archiveReqs := first.counts()
	require.Equal(t, 1, head)
	require.Equal(t, 1, checksumReqs)
	require.Equal(t, 1, archiveReqs)
	head, checksumReqs, archiveReqs = second.counts()
	require.Equal(t, 1, head)
	require.Zero(t, checksumReqs)
	require.Zero(t, archiveReqs)
}

func TestUpdateChecksumMismatchFallback(t *testing.T) {
	t.Parallel()

	asset, err := assetName(runtime.GOOS, runtime.GOARCH, buildGOARM())
	require.NoError(t, err)
	archive, checksum := makeReleaseArchive(t, "v1.2.3", binaryNameForHost())
	first := newFakeRelease(t, asset, archive, checksum)
	first.checksums = strings.Repeat("0", 64) + "  " + asset + "\n"
	second := newFakeRelease(t, asset, archive, checksum)

	u, _, errOut := newUpdateHarness(t)
	u.httpClient = first.server.Client()
	target := filepath.Join(t.TempDir(), "ncmctl")
	writeVersionScript(t, target, "v1.0.0")

	routes := []string{first.server.URL + "/", second.server.URL + "/"}

	err = u.run(context.Background(), &updateState{routes: routes, target: target})
	require.NoError(t, err)
	require.Equal(t, "v1.2.3", mustVersion(t, target))
	require.Contains(t, errOut.String(), "SHA-256 verification failed via "+routeName(routes[0])+".")

	_, _, archiveReqs := first.counts()
	require.Equal(t, 1, archiveReqs)
}

func TestUpdateChecksumMismatchAllFail(t *testing.T) {
	t.Parallel()

	s := newUpdateScenario(t, "v1.0.0")
	s.f.checksums = strings.Repeat("0", 64) + "  " + s.f.asset + "\n"

	err := s.run(t)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unable to download a valid "+s.f.asset+" from any configured route")
	require.Contains(t, s.errOut.String(), "SHA-256 verification failed via "+routeName(s.f.server.URL+"/")+".")
	require.Equal(t, "v1.0.0", mustVersion(t, s.target))

	leftovers, globErr := filepath.Glob(filepath.Join(filepath.Dir(s.target), ".ncmctl.update-*"))
	require.NoError(t, globErr)
	require.Empty(t, leftovers)

	_, statErr := os.Stat(filepath.Join(filepath.Dir(s.target), installLockName))
	require.True(t, os.IsNotExist(statErr))
}

func TestUpdateBinaryVersionMismatchFallback(t *testing.T) {
	t.Parallel()

	asset, err := assetName(runtime.GOOS, runtime.GOARCH, buildGOARM())
	require.NoError(t, err)
	staleArchive, staleChecksum := makeReleaseArchive(t, "v0.0.0", binaryNameForHost())
	archive, checksum := makeReleaseArchive(t, "v1.2.3", binaryNameForHost())
	first := newFakeRelease(t, asset, staleArchive, staleChecksum)
	second := newFakeRelease(t, asset, archive, checksum)

	u, _, errOut := newUpdateHarness(t)
	u.httpClient = first.server.Client()
	target := filepath.Join(t.TempDir(), "ncmctl")
	writeVersionScript(t, target, "v1.0.0")

	routes := []string{first.server.URL + "/", second.server.URL + "/"}

	err = u.run(context.Background(), &updateState{routes: routes, target: target})
	require.NoError(t, err)
	require.Equal(t, "v1.2.3", mustVersion(t, target))
	require.Contains(t, errOut.String(), "Downloaded binary version v0.0.0 does not match release v1.2.3 via "+routeName(routes[0])+".")
}

func TestUpdateChecksumManifestVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest func(checksum, asset string) string
		errMsg   string
	}{
		{name: "uppercase normalized", manifest: func(checksum, asset string) string {
			return strings.ToUpper(checksum) + "  " + asset + "\n"
		}},
		{name: "short hash rejected", manifest: func(_, asset string) string {
			return strings.Repeat("a", 63) + "  " + asset + "\n"
		}, errMsg: "Unable to download a valid"},
		{name: "non hex hash rejected", manifest: func(_, asset string) string {
			return strings.Repeat("g", 64) + "  " + asset + "\n"
		}, errMsg: "Unable to download a valid"},
		{name: "missing entry rejected", manifest: func(checksum, _ string) string {
			return checksum + "  ncmctl_Linux_arm64.tar.gz\n"
		}, errMsg: "Unable to download a valid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newUpdateScenario(t, "v1.0.0")
			s.f.checksums = tt.manifest(s.f.checksum, s.f.asset)

			err := s.run(t)
			if tt.errMsg != "" {
				require.ErrorContains(t, err, tt.errMsg)
				return
			}

			require.NoError(t, err)
			require.Equal(t, "v1.2.3", mustVersion(t, s.target))
		})
	}
}

func TestUpdateLockContention(t *testing.T) {
	t.Parallel()

	s := newUpdateScenario(t, "v1.0.0")
	lockDir := filepath.Join(filepath.Dir(s.resolvedTarget(t)), installLockName)
	require.NoError(t, os.Mkdir(lockDir, 0o750))

	err := s.run(t)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unable to acquire installation lock "+lockDir)
	require.DirExists(t, lockDir)
	require.Equal(t, "v1.0.0", mustVersion(t, s.target))
}

func TestUpdateReadOnlyTargetDir(t *testing.T) {
	t.Parallel()

	s := newUpdateScenario(t, "v1.0.0")
	targetDir := filepath.Dir(s.resolvedTarget(t))
	require.NoError(t, os.Chmod(targetDir, 0o500)) //nolint:gosec // The test deliberately weakens directory permissions.
	t.Cleanup(func() {
		_ = os.Chmod(targetDir, 0o700) //nolint:gosec // The test deliberately weakens directory permissions.
	})

	if err := os.WriteFile(filepath.Join(targetDir, "probe"), []byte("x"), 0o600); err == nil {
		t.Skip("running as root; directory remains writable")
	}

	err := s.run(t)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unable to download a valid "+s.f.asset+" from any configured route")
	require.Contains(t, s.errOut.String(), "Unable to stage "+binaryNameForHost()+" in "+targetDir)
	require.Contains(t, s.errOut.String(), "Manual upgrade: "+releasesURL)
	require.Equal(t, "v1.0.0", mustVersion(t, s.target))
}

func TestUpdateConcurrentUpgradeRefused(t *testing.T) {
	t.Parallel()

	s := newUpdateScenario(t, "v1.0.0")
	st := &updateState{
		routes:    []string{s.f.server.URL + "/"},
		target:    s.target,
		targetDir: filepath.Dir(s.target),
		binary:    binaryNameForHost(),
		tempDir:   t.TempDir(),
		latest:    "v1.2.3",
	}
	asset, err := assetName(runtime.GOOS, runtime.GOARCH, buildGOARM())
	require.NoError(t, err)

	st.asset = asset

	staged, err := s.u.downloadAndVerify(context.Background(), s.u.httpClient, st)
	require.NoError(t, err)

	st.staged = staged

	writeVersionScript(t, s.target, "v2.0.0")
	require.NoError(t, s.u.install(st))
	require.Equal(t, "v2.0.0", mustVersion(t, s.target))

	_, statErr := os.Stat(staged)
	require.NoError(t, statErr)
	require.Contains(t, s.out.String(), "Installed version v2.0.0 is newer than GitHub Release v1.2.3; skipping downgrade.")
}

func TestUpdateCancellation(t *testing.T) {
	t.Parallel()

	s := newUpdateScenario(t, "v1.0.0")
	st := &updateState{routes: []string{s.f.server.URL + "/"}, target: s.target}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.u.run(ctx, st)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unable to resolve the latest GitHub release from any configured route")

	_, statErr := os.Stat(st.tempDir)
	require.True(t, os.IsNotExist(statErr))

	leftovers, globErr := filepath.Glob(filepath.Join(filepath.Dir(s.target), ".ncmctl.update-*"))
	require.NoError(t, globErr)
	require.Empty(t, leftovers)

	_, statErr = os.Stat(filepath.Join(filepath.Dir(s.target), installLockName))
	require.True(t, os.IsNotExist(statErr))
}

func TestUpdateSymlinkExecutable(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires privileges on Windows")
	}

	s := newUpdateScenario(t, "v1.0.0")
	link := filepath.Join(t.TempDir(), "ncmctl")
	require.NoError(t, os.Symlink(s.target, link))

	err := s.u.run(context.Background(), &updateState{routes: []string{s.f.server.URL + "/"}, target: link})
	require.NoError(t, err)
	require.Equal(t, "v1.2.3", mustVersion(t, s.target))
	require.Equal(t, "v1.2.3", mustVersion(t, link))
	readlink, readErr := os.Readlink(link)
	require.NoError(t, readErr)
	require.Equal(t, s.target, readlink)
}

func TestUpdateDirectOnly(t *testing.T) {
	t.Parallel()

	u, _, _ := newUpdateHarness(t)
	require.NoError(t, u.cmd.Flags().Set("proxy", ""))

	rt := &recordingRoundTripper{}
	u.httpClient = &http.Client{Transport: rt}

	err := u.run(context.Background(), &updateState{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unable to resolve the latest GitHub release from any configured route")

	rt.mu.Lock()
	urls := append([]string(nil), rt.urls...)
	rt.mu.Unlock()
	require.Equal(t, []string{releaseLatestURL}, urls)
}

func TestUpdateInvalidProxy(t *testing.T) {
	t.Parallel()

	u, _, _ := newUpdateHarness(t)
	require.NoError(t, u.cmd.Flags().Set("proxy", "http://user:pass@host"))

	rt := &recordingRoundTripper{}
	u.httpClient = &http.Client{Transport: rt}

	err := u.run(context.Background(), &updateState{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid HTTPS proxy at position 1: scheme must be https")

	rt.mu.Lock()
	require.Empty(t, rt.urls)
	rt.mu.Unlock()
}

type updateScenario struct {
	u      *Update
	out    *bytes.Buffer
	errOut *bytes.Buffer
	f      *fakeRelease
	target string
}

func newUpdateScenario(t *testing.T, installedVersion string) *updateScenario {
	t.Helper()

	u, out, errOut := newUpdateHarness(t)
	asset, err := assetName(runtime.GOOS, runtime.GOARCH, buildGOARM())
	require.NoError(t, err)
	archive, checksum := makeReleaseArchive(t, "v1.2.3", binaryNameForHost())
	f := newFakeRelease(t, asset, archive, checksum)
	u.httpClient = f.server.Client()
	target := filepath.Join(t.TempDir(), "ncmctl")
	writeVersionScript(t, target, installedVersion)

	return &updateScenario{u: u, out: out, errOut: errOut, f: f, target: target}
}

func (s *updateScenario) run(t *testing.T) error {
	t.Helper()

	return s.u.run(context.Background(), &updateState{routes: []string{s.f.server.URL + "/"}, target: s.target})
}

// resolvedTarget mirrors the EvalSymlinks run() applies to the target, which
// resolves /var to /private/var on macOS.
func (s *updateScenario) resolvedTarget(t *testing.T) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(s.target)
	require.NoError(t, err)
	return resolved
}

func newUpdateHarness(t *testing.T) (*Update, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	u := NewUpdate(&Root{})

	var out, errOut bytes.Buffer
	u.cmd.SetOut(&out)
	u.cmd.SetErr(&errOut)
	return u, &out, &errOut
}

func binaryNameForHost() string {
	if runtime.GOOS == "windows" {
		return "ncmctl.exe"
	}
	return "ncmctl"
}

func mustVersion(t *testing.T, path string) string {
	t.Helper()

	version, err := binaryVersion(path)
	require.NoError(t, err)
	return version
}

func versionScript(version string) string {
	return "#!/usr/bin/env bash\n" +
		"echo \"ncmctl\"\n" +
		"echo \" Version: \t" + version + "\"\n" +
		"echo \" Go version: \tgo1.25.0\"\n" +
		"echo \" Git commit: \ttest\"\n" +
		"echo \" OS/Arch: \ttest\"\n" +
		"echo \" Build time: \ttest\"\n"
}

func writeVersionScript(t *testing.T, path, version string) {
	t.Helper()

	writeRawScript(t, path, versionScript(version))
}

func writeRawScript(t *testing.T, path, body string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	require.NoError(t, os.Chmod(path, 0o755)) // #nosec G302 -- test fixture needs the executable bit
}

type archiveEntry struct {
	name    string
	content []byte
	mode    int64
	symlink string
}

func makeTarGz(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()

	var buf bytes.Buffer

	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for _, e := range entries {
		if e.symlink != "" {
			hdr := &tar.Header{Name: e.name, Typeflag: tar.TypeSymlink, Linkname: e.symlink}
			require.NoError(t, tw.WriteHeader(hdr))
			continue
		}

		hdr := &tar.Header{Name: e.name, Mode: e.mode, Size: int64(len(e.content))}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write(e.content)
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func makeZip(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()

	var buf bytes.Buffer

	zw := zip.NewWriter(&buf)

	for _, e := range entries {
		hdr := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		hdr.SetMode(fs.FileMode(e.mode))
		w, err := zw.CreateHeader(hdr)
		require.NoError(t, err)
		_, err = w.Write(e.content)
		require.NoError(t, err)
	}

	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func makeReleaseArchive(t *testing.T, version, binaryName string) ([]byte, string) {
	t.Helper()

	entries := []archiveEntry{{name: binaryName, mode: 0o755, content: []byte(versionScript(version))}}

	var archive []byte
	if runtime.GOOS == "windows" {
		archive = makeZip(t, entries)
	} else {
		archive = makeTarGz(t, entries)
	}

	sum := sha256.Sum256(archive)
	return archive, hex.EncodeToString(sum[:])
}

type fakeRelease struct {
	mu           sync.Mutex
	version      string
	asset        string
	archive      []byte
	checksum     string
	checksums    string
	failHead     bool
	headReqs     int
	checksumReqs int
	archiveReqs  int
	server       *httptest.Server
}

func (f *fakeRelease) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	checksumsName := "ncmctl_" + f.version + "_checksums.txt"
	switch r.URL.Path {
	case testLatestPath:
		f.headReqs++
		if f.failHead {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, f.server.URL+testTagPath+"v"+f.version, http.StatusFound)
	case testTagPath + "v" + f.version:
		http.Error(w, "ok", http.StatusOK)
	case testDownloadPath + "v" + f.version + "/" + checksumsName:
		f.checksumReqs++
		if f.checksums != "" {
			_, _ = w.Write([]byte(f.checksums))
			return
		}

		_, _ = w.Write([]byte(f.checksum + "  " + f.asset + "\n"))
	case testDownloadPath + "v" + f.version + "/" + f.asset:
		f.archiveReqs++
		_, _ = w.Write(f.archive)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeRelease) counts() (int, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.headReqs, f.checksumReqs, f.archiveReqs
}

func newFakeRelease(t *testing.T, asset string, archive []byte, checksum string) *fakeRelease {
	t.Helper()

	f := &fakeRelease{version: "1.2.3", asset: asset, archive: archive, checksum: checksum}
	f.server = httptest.NewTLSServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)
	return f
}

type recordingRoundTripper struct {
	mu   sync.Mutex
	urls []string
}

func (r *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.urls = append(r.urls, req.URL.String())
	r.mu.Unlock()
	return nil, errors.New("blocked by recording round tripper")
}
