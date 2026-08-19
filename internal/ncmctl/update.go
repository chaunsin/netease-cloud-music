// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

const (
	githubRepoURL         = "https://github.com/chaunsin/netease-cloud-music"
	releaseLatestURL      = githubRepoURL + "/releases/latest"
	releaseTagPrefix      = githubRepoURL + "/releases/tag/"
	releaseDownloadPrefix = githubRepoURL + "/releases/download/"
	releasesURL           = githubRepoURL + "/releases"
	installLockName       = ".ncmctl.install.lock"

	connectTimeout  = 10 * time.Second
	metadataTimeout = 30 * time.Second
	archiveTimeout  = 300 * time.Second

	checksumsMaxSize = 1 << 20   // 1 MiB
	archiveMaxSize   = 256 << 20 // 256 MiB
)

var (
	defaultGithubProxies = []string{"https://ghproxy.net/", "https://ghfast.top/", "https://gh-proxy.com/"}
	semverPattern        = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)
)

var (
	errNoBinaryEntry = errors.New("archive does not contain the binary entry")
	errCreateStaging = errors.New("cannot create the staging file")
	errEntryTooLarge = errors.New("archive entry exceeds the size limit")
	removeFile       = os.Remove

	// windowsReplace 按 runtime.GOOS 初始化，测试中可临时覆盖以在任意平台
	// 验证 replaceExecutable 的 Windows 分支（与 removeFile 注入同模式）。
	windowsReplace = runtime.GOOS == "windows"
)

type UpdateOpts struct {
	Proxy string
}

type Update struct {
	root       *Root
	cmd        *cobra.Command
	opts       UpdateOpts
	httpClient *http.Client
}

func NewUpdate(root *Root) *Update {
	c := &Update{
		root: root,
		cmd: &cobra.Command{
			Use:   "update",
			Short: "Update ncmctl to the latest GitHub release",
			Long: "Download the latest ncmctl release from GitHub Releases, verify its SHA-256 checksum, " +
				"and replace the currently running executable in place. " +
				"Release assets are fetched through the built-in proxy chain " +
				"(https://ghproxy.net/, https://ghfast.top/, https://gh-proxy.com/) with direct GitHub access as the final fallback; " +
				"--proxy overrides the chain, and an empty value forces direct access. " +
				"SHA-256 verification is mandatory, and the installed version is never downgraded.",
			Example: "  ncmctl update\n" +
				"  ncmctl update --proxy \"https://proxy.example/ https://mirror.example/\"\n" +
				"  ncmctl update --proxy \"\"",
			Args: cobra.NoArgs,
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context(), args)
	}

	return c
}

func (c *Update) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Update) Command() *cobra.Command {
	return c.cmd
}

func (c *Update) addFlags() {
	c.cmd.Flags().StringVar(
		&c.opts.Proxy,
		"proxy",
		"",
		"space-separated HTTPS proxy prefixes for release downloads; pass an empty value (--proxy \"\") to force direct GitHub access (default: built-in proxy chain)",
	)
}

// execute 为命令建立信号上下文：收到 SIGINT/SIGTERM 时取消 ctx，
// 使进行中的下载被中断，由 run 的清理逻辑移除临时文件后退出。
func (c *Update) execute(ctx context.Context, _ []string) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	return c.run(ctx, &updateState{})
}

type updateState struct {
	routes    []string
	latest    string
	asset     string
	binary    string
	tempDir   string
	target    string
	targetDir string
	lockDir   string
	staged    string
}

// cleanup 按依赖顺序清理运行产物：先删暂存二进制，再释放安装锁，最后
// 删除下载临时目录；staged/lockDir/tempDir 均只在非空时处理，可安全重复调用。
func (st *updateState) cleanup() {
	if st.staged != "" {
		_ = os.Remove(st.staged)
	}

	st.releaseLock()

	if st.tempDir != "" {
		_ = os.RemoveAll(st.tempDir)
	}
}

// releaseLock clears the ownership flag before removing the directory so that
// a stale lock can never be deleted twice by the same state.
func (st *updateState) releaseLock() {
	if st.lockDir == "" {
		return
	}

	dir := st.lockDir
	st.lockDir = ""
	_ = os.Remove(dir)
}

// run 编排升级流程：构建路由链、解析当前可执行文件的真实路径、获取最新
// 版本、判断是否需要安装、下载并校验归档，最后在安装锁内替换二进制。
// EvalSymlinks 把 /var 之类的符号链接解析为真实路径，避免替换前后目标
// 路径不一致（如 macOS 上 /var 与 /private/var）。
func (c *Update) run(ctx context.Context, st *updateState) error {
	defer st.cleanup()

	if st.routes == nil {
		routes, err := buildRoutes(c.opts.Proxy, c.cmd.Flags().Changed("proxy"))
		if err != nil {
			return err
		}

		st.routes = routes
	}

	if st.target == "" {
		target, err := resolveExecutable()
		if err != nil {
			return err
		}

		st.target = target
	} else if resolved, err := filepath.EvalSymlinks(st.target); err == nil {
		st.target = resolved
	}

	st.targetDir = filepath.Dir(st.target)

	st.binary = "ncmctl"
	if runtime.GOOS == "windows" {
		st.binary = "ncmctl.exe"
	}

	asset, err := assetName(runtime.GOOS, runtime.GOARCH, buildGOARM())
	if err != nil {
		return err
	}

	st.asset = asset
	if st.tempDir == "" {
		st.tempDir, err = os.MkdirTemp("", "ncmctl_update-*")
		if err != nil {
			return fmt.Errorf("create temp dir: %w", err)
		}
	}

	client := c.httpClient
	if client == nil {
		client = newUpdateHTTPClient()
	}

	st.latest, err = c.resolveLatestVersion(ctx, client, st.routes)
	if err != nil {
		return err
	}

	if !c.checkUpToDate(st) {
		return nil
	}

	st.staged, err = c.downloadAndVerify(ctx, client, st)
	if err != nil {
		return err
	}
	return c.install(st)
}

// checkUpToDate 比较已安装版本与最新版本，返回 true 表示需要继续安装
// （可升级、版本无法读取或不是合法 SemVer），false 表示无需操作
// （已是最新或拒绝降级）。已安装版本通过执行目标二进制 --version 获取。
func (c *Update) checkUpToDate(st *updateState) bool {
	out, err := binaryVersion(st.target)
	if err != nil {
		c.cmd.PrintErrln("Unable to read the installed version; reinstalling.")
		return true
	}

	installed, err := parseSemver(out)
	if err != nil {
		c.cmd.PrintErrf("Installed version %s is not valid SemVer; reinstalling %s.\n", out, st.latest)
		return true
	}

	latest, err := parseSemver(st.latest)
	if err != nil {
		c.cmd.PrintErrf("Installed version %s is not valid SemVer; reinstalling %s.\n", st.latest, st.latest)
		return true
	}

	switch installed.Compare(latest) {
	case 0:
		c.cmd.Printf("ncmctl is up-to-date (version: %s).\n", out)
		return false
	case 1:
		c.cmd.Printf("Installed version %s is newer than GitHub Release %s; skipping downgrade.\n", out, st.latest)
		return false
	default:
		c.cmd.Printf("Installed version: %s. A newer version (%s) is available.\n", out, st.latest)
		return true
	}
}

// resolveLatestVersion 依次尝试每个路由：对 /releases/latest 发 HEAD 请求，
// GitHub 会 302 重定向到 /releases/tag/vX.Y.Z，版本号从最终 URL 提取；
// 单个路由失败仅告警并回退到下一个，全部失败才返回错误。
func (c *Update) resolveLatestVersion(ctx context.Context, client *http.Client, routes []string) (string, error) {
	c.cmd.Println("Fetching the latest release tag from GitHub...")

	for _, prefix := range routes {
		name := routeName(prefix)
		c.cmd.Println("Trying " + name + "...")

		requestCtx, cancel := context.WithTimeout(ctx, metadataTimeout)

		req, err := http.NewRequestWithContext(requestCtx, http.MethodHead, prefix+releaseLatestURL, http.NoBody)
		if err != nil {
			cancel()
			c.cmd.PrintErrln("Failed to resolve the latest release tag via " + name + ".")
			continue
		}

		resp, err := client.Do(req)

		cancel()

		if err != nil {
			c.cmd.PrintErrln("Failed to resolve the latest release tag via " + name + ".")
			continue
		}

		version, verErr := versionFromReleaseURL(resp.Request.URL.String(), prefix)
		statusOK := resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
		_ = resp.Body.Close()

		if verErr != nil || !statusOK {
			c.cmd.PrintErrln("Failed to resolve the latest release tag via " + name + ".")
			continue
		}

		c.cmd.Println("Latest version: " + version + ".")
		return version, nil
	}
	return "", errors.New("Unable to resolve the latest GitHub release from any configured route") //nolint:staticcheck // Spec-mandated message for parity with the reference installer script.
}

// downloadAndVerify 按路由依次下载并校验发布资产：先取 checksums 清单并
// 扫描目标资产的条目（每行格式为 "<64位十六进制>  <资产名>"，校验通过后统一
// 转小写再与本地计算值比较），随后下载归档并核对 SHA-256；提取后还会执行
// 一次新二进制并比对 --version 输出，防止下载到被篡改或版本不符的构建。
// 任一环节失败都清理半成品并回退到下一个路由。
func (c *Update) downloadAndVerify(ctx context.Context, client *http.Client, st *updateState) (string, error) {
	var (
		tag           = strings.TrimPrefix(st.latest, "v")
		checksumsName = "ncmctl_" + tag + "_checksums.txt"
		checksumsURL  = releaseDownloadPrefix + st.latest + "/" + checksumsName
		archiveURL    = releaseDownloadPrefix + st.latest + "/" + st.asset
		checksumsPath = filepath.Join(st.tempDir, checksumsName+".part")
		archivePath   = filepath.Join(st.tempDir, st.asset+".part")
	)

	for _, prefix := range st.routes {
		name := routeName(prefix)

		c.cmd.Println("Downloading release checksums via " + name + "...")

		if err := c.downloadFile(ctx, client, prefix+checksumsURL, checksumsPath, metadataTimeout, checksumsMaxSize); err != nil {
			c.cmd.PrintErrln("Failed to download release checksums via " + name + ".")
			continue
		}

		f, err := os.Open(checksumsPath)
		if err != nil {
			_ = os.Remove(checksumsPath)

			c.cmd.PrintErrln("Checksum entry for " + st.asset + " was not found via " + name + ".")
			continue
		}

		expected := ""

		for scanner := bufio.NewScanner(f); scanner.Scan(); {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 2 || fields[1] != st.asset || len(fields[0]) != 64 {
				continue
			}

			if _, hexErr := hex.DecodeString(fields[0]); hexErr != nil {
				continue
			}

			expected = strings.ToLower(fields[0])
			break
		}

		_ = f.Close()

		if expected == "" {
			_ = os.Remove(checksumsPath)

			c.cmd.PrintErrln("Checksum entry for " + st.asset + " was not found via " + name + ".")
			continue
		}

		c.cmd.Println("Downloading " + st.asset + " via " + name + "...")

		if err = c.downloadFile(ctx, client, prefix+archiveURL, archivePath, archiveTimeout, archiveMaxSize); err != nil {
			c.cmd.PrintErrln("Download failed via " + name + ".")
			continue
		}

		actual, err := sha256File(archivePath)
		if err != nil || actual != expected {
			_ = os.Remove(archivePath)

			c.cmd.PrintErrln("SHA-256 verification failed via " + name + ".")
			continue
		}

		staged, err := extractBinaryFromArchive(archivePath, st.targetDir, st.binary, archiveMaxSize)
		_ = os.Remove(archivePath)

		if err != nil {
			switch {
			case errors.Is(err, errNoBinaryEntry):
				c.cmd.PrintErrln("Archive does not contain " + st.binary + " via " + name + ".")
			case errors.Is(err, errCreateStaging):
				c.cmd.PrintErrln("Unable to stage " + st.binary + " in " + st.targetDir + "; Manual upgrade: " + releasesURL)
			case errors.Is(err, errEntryTooLarge):
				c.cmd.PrintErrln("Archive entry for " + st.binary + " exceeds the size limit via " + name + ".")
			default:
				c.cmd.PrintErrln("Failed to extract " + st.binary + " via " + name + ".")
			}
			continue
		}

		version, err := binaryVersion(staged)
		if err != nil {
			_ = os.Remove(staged)

			c.cmd.PrintErrln("Downloaded binary cannot report its version via " + name + ".")
			continue
		}

		if strings.TrimPrefix(version, "v") != tag {
			_ = os.Remove(staged)

			c.cmd.PrintErrln("Downloaded binary version " + version + " does not match release " + st.latest + " via " + name + ".")
			continue
		}
		return staged, nil
	}
	return "", fmt.Errorf("Unable to download a valid %s from any configured route", st.asset) //nolint:staticcheck // Spec-mandated message for parity with the reference installer script.
}

// downloadFile 带超时下载单个文件到 dest；非 2xx 状态、超出大小上限或写入
// 失败时删除半成品文件，避免残留的 .part 影响后续路由重试。
func (c *Update) downloadFile(ctx context.Context, client *http.Client, requestURL, dest string, timeout time.Duration, maxBytes int64) error {
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	if resp.ContentLength > maxBytes {
		return fmt.Errorf("response exceeds the %d byte size limit", maxBytes)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}

	written, err := io.Copy(f, io.LimitReader(resp.Body, maxBytes+1))
	if err == nil && written > maxBytes {
		err = fmt.Errorf("response exceeds the %d byte size limit", maxBytes)
	}

	if closeErr := f.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		_ = os.Remove(dest)
	}
	return err
}

// install 在目标目录用 os.Mkdir 抢占安装锁：Mkdir 对同一路径具备原子性，
// 成功即代表持锁，失败说明已有其他安装器在运行。持锁后锁内复查一次
// checkUpToDate，抵御两个进程并发升级同一二进制（下载阶段不持锁）。
func (c *Update) install(st *updateState) error {
	lockDir := filepath.Join(st.targetDir, installLockName)
	if err := os.Mkdir(lockDir, 0o750); err != nil {
		return fmt.Errorf( //nolint:staticcheck // Spec-mandated message for parity with the reference installer script.
			"Unable to acquire installation lock %s; another installer may be running (remove a stale lock only after verifying no installer is active)",
			lockDir,
		)
	}

	st.lockDir = lockDir
	defer st.releaseLock()

	if !c.checkUpToDate(st) {
		return nil
	}

	if err := c.replaceExecutable(st.target, st.staged); err != nil {
		return fmt.Errorf("%w; Manual upgrade: %s", err, releasesURL)
	}

	st.staged = ""
	c.cmd.Println("ncmctl installed successfully at " + st.target + " (version: " + st.latest + ").")
	c.cmd.Println("Restart ncmctl to use the new version.")
	return nil
}

// replaceExecutable 替换当前二进制：Unix 直接 os.Rename 原子覆盖（运行中
// 可替换）；Windows 上运行中的进程会占用旧 exe，走三步替换——移开旧文件、
// 换入新文件、删除旧文件。其中 target 缺失（上次替换中途失败）时跳过移开
// 步骤直接换入自愈，否则后续升级都会卡在移开步骤无法恢复；.old 删除失败
// （运行中必然）时新二进制已经就位，仅告警并视为成功，残留文件由下次升级
// 覆盖。
func (c *Update) replaceExecutable(target, staged string) error {
	if !windowsReplace {
		return os.Rename(staged, target)
	}

	if _, err := os.Stat(target); os.IsNotExist(err) {
		return os.Rename(staged, target)
	}

	old := target + ".old"
	if err := os.Rename(target, old); err != nil {
		return fmt.Errorf("move current binary aside: %w", err)
	}

	if err := os.Rename(staged, target); err != nil {
		if _, statErr := os.Stat(target); os.IsNotExist(statErr) {
			if rollbackErr := os.Rename(old, target); rollbackErr == nil {
				return fmt.Errorf("stage new binary: %w", err)
			}
		}
		return fmt.Errorf("stage new binary: %w", err)
	}

	if err := removeFile(old); err != nil {
		c.cmd.PrintErrf("The previous binary could not be removed (%s); the leftover will be replaced by the next update.\n", old)
	}
	return nil
}

// buildRoutes 构造路由链：未指定代理时使用内置镜像列表；末尾追加空前缀
// 路由作为 GitHub 直连兜底，保证所有代理失效时仍可重试直连。
func buildRoutes(proxy string, proxyChanged bool) ([]string, error) {
	prefixes := defaultGithubProxies
	if proxyChanged {
		prefixes = strings.Fields(proxy)
	}

	routes := make([]string, 0, len(prefixes)+1)
	for i, prefix := range prefixes {
		normalized, err := validateProxy(prefix)
		if err != nil {
			return nil, fmt.Errorf("invalid HTTPS proxy at position %d: %w", i+1, err)
		}

		routes = append(routes, normalized)
	}
	return append(routes, ""), nil
}

// validateProxy 校验并规范化代理前缀：拒绝非 HTTPS、带用户信息、缺主机或
// 端口非法的前缀；统一以单斜杠结尾，保证与 releaseLatestURL 等绝对 URL
// 直接拼接后仍是合法地址。
func validateProxy(prefix string) (string, error) {
	u, err := url.Parse(prefix)
	if err != nil {
		return "", errors.New("not a valid URL")
	}

	if u.Scheme != "https" {
		return "", errors.New("scheme must be https")
	}

	if u.User != nil {
		return "", errors.New("userinfo is not allowed")
	}

	if u.Host == "" || u.Hostname() == "" {
		return "", errors.New("missing host")
	}

	if strings.TrimSpace(u.Hostname()) != u.Hostname() {
		return "", errors.New("invalid hostname")
	}

	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", errors.New("invalid port")
		}
	}
	return strings.TrimRight(prefix, "/") + "/", nil
}

// routeName 生成日志中使用的路由名，只暴露主机名与端口，避免把前缀路径
// 中的访问令牌等敏感信息打印到日志。
func routeName(prefix string) string {
	if prefix == "" {
		return "GitHub"
	}

	u, err := url.Parse(prefix)
	if err != nil || u.Host == "" || strings.TrimSpace(u.Host) != u.Host {
		return "configured HTTPS proxy"
	}
	return "HTTPS proxy " + u.Host
}

// semver holds a parsed SemVer 2.0.0 version; build metadata is stripped
// because it never participates in precedence.
type semver struct {
	core string
	pre  string
}

func parseSemver(version string) (*semver, error) {
	v := strings.TrimPrefix(version, "v")
	if !semverPattern.MatchString(v) {
		return nil, errors.New("invalid SemVer")
	}

	core, pre := v, ""
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		core = v[:i]

		if after, ok := strings.CutPrefix(v[i:], "-"); ok {
			if pre = after; strings.Contains(pre, "+") {
				pre = strings.SplitN(pre, "+", 2)[0]
			}
		}
	}
	return &semver{core: core, pre: pre}, nil
}

// Compare returns -1, 0 or 1 following the SemVer 2.0.0 precedence rules.
func (v *semver) Compare(o *semver) int {
	if cmp := v.compareCore(o); cmp != 0 {
		return cmp
	}

	switch {
	case v.pre == "" && o.pre == "":
		return 0
	case v.pre == "":
		return 1
	case o.pre == "":
		return -1
	default:
		return v.comparePrerelease(o)
	}
}

func (v *semver) compareCore(o *semver) int {
	left := strings.Split(v.core, ".")

	right := strings.Split(o.core, ".")
	for i := range left {
		if cmp := v.compareDecimal(left[i], right[i]); cmp != 0 {
			return cmp
		}
	}
	return 0
}

func (v *semver) comparePrerelease(o *semver) int {
	left := strings.Split(v.pre, ".")

	right := strings.Split(o.pre, ".")
	for i := 0; i < len(left) && i < len(right); i++ {
		if cmp := v.compareIdentifier(left[i], right[i]); cmp != 0 {
			return cmp
		}
	}

	switch {
	case len(left) < len(right):
		return -1
	case len(left) > len(right):
		return 1
	default:
		return 0
	}
}

func (v *semver) compareDecimal(left, right string) int {
	left = strings.TrimLeft(left, "0")

	right = strings.TrimLeft(right, "0")
	switch {
	case len(left) < len(right):
		return -1
	case len(left) > len(right):
		return 1
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func (v *semver) compareIdentifier(left, right string) int {
	leftNumeric := isNumeric(left)

	rightNumeric := isNumeric(right)
	switch {
	case leftNumeric && rightNumeric:
		return v.compareDecimal(left, right)
	case leftNumeric:
		return -1
	case rightNumeric:
		return 1
	default:
		return strings.Compare(left, right)
	}
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}

	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func assetName(goos, goarch, goarm string) (string, error) {
	var osName string

	switch goos {
	case "linux":
		osName = "Linux"
	case "darwin":
		osName = "Darwin"
	case "windows":
		osName = "Windows"
	case "freebsd":
		osName = "Freebsd"
	case "openbsd":
		osName = "Openbsd"
	case "netbsd":
		osName = "Netbsd"
	default:
		return "", fmt.Errorf("unsupported GOOS: %s", goos)
	}

	var archName string

	switch goarch {
	case "amd64":
		archName = "x86_64"
	case "386":
		archName = "i386"
	case "arm":
		if goarm != "6" {
			return "", fmt.Errorf("unsupported GOARM: %s", goarm)
		}

		archName = "armv6"
	case "arm64", "s390x", "ppc64", "ppc64le", "riscv64", "mips", "mipsle", "mips64", "mips64le", "loong64":
		archName = goarch
	default:
		return "", fmt.Errorf("unsupported GOARCH: %s", goarch)
	}

	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return "ncmctl_" + osName + "_" + archName + "." + ext, nil
}

// versionFromReleaseURL 从重定向后的最终 URL 提取版本号，兼容两种形态：
// 一是代理前缀拼接出的 GitHub 直链（proxy.example/https://github.com/.../tag/v1.2.3），
// 二是代理把请求原样回弹给 GitHub 后的直连 tag 链接。两种情况都要求仓库
// 路径与本项目的 tag 前缀完全一致，外来仓库或多余路径一律拒绝。
func versionFromReleaseURL(releaseURL, prefix string) (string, error) {
	if after, ok := strings.CutPrefix(releaseURL, prefix+releaseTagPrefix); ok {
		version, err := versionFromTag(after)
		if err != nil {
			return "", err
		}
		return version, nil
	}

	if prefix != "" && strings.HasPrefix(releaseURL, releaseTagPrefix) {
		version, err := versionFromTag(strings.TrimPrefix(releaseURL, releaseTagPrefix))
		if err != nil {
			return "", err
		}
		return version, nil
	}
	return "", errors.New("unexpected release URL")
}

// versionFromTag 要求 tag 带 v 前缀且剩余部分是严格 SemVer，防止恶意或
// 损坏的发布页把任意字符串当作版本号进入后续比较与下载流程。
func versionFromTag(tag string) (string, error) {
	if !strings.HasPrefix(tag, "v") || !semverPattern.MatchString(strings.TrimPrefix(tag, "v")) {
		return "", errors.New("invalid release tag")
	}
	return tag, nil
}

// binaryVersion 执行目标二进制 --version 并扫描输出中的 "Version:" 行；
// 行首允许任意空白、行尾容忍 CRLF，以兼容带 ASCII art 等额外内容的输出。
func binaryVersion(binaryPath string) (string, error) {
	out, err := exec.Command(binaryPath, "--version").Output()
	if err != nil {
		return "", err
	}

	for line := range strings.SplitSeq(string(out), "\n") {
		rest, found := strings.CutPrefix(strings.TrimLeft(line, " \t"), "Version:")
		if found && strings.TrimSpace(rest) != "" {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", errors.New("no Version line found")
}

// buildGOARM reports the GOARM build setting, which Go does not expose as a
// runtime constant; only relevant for the 32-bit ARM port.
func buildGOARM() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}

	for _, setting := range info.Settings {
		if setting.Key == "GOARM" {
			return setting.Value
		}
	}
	return ""
}

// resolveExecutable 返回当前可执行文件的真实路径：os.Executable 在部分
// 系统上返回符号链接路径（如 macOS 的 /var → /private/var），解析后保证
// 替换与清理始终作用于同一真实文件。
func resolveExecutable() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}

	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		return resolved, nil
	}
	return exePath, nil
}

// newUpdateHTTPClient 构造升级流程专用的客户端：显式禁用环境代理（路由链
// 由 buildRoutes 决定，避免系统代理破坏回退语义），并禁止重定向到非 HTTPS。
func newUpdateHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:               nil,
			DialContext:         (&net.Dialer{Timeout: connectTimeout}).DialContext,
			TLSHandshakeTimeout: connectTimeout,
		},
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if req.URL.Scheme != "https" {
				return errors.New("redirect to a non-HTTPS URL")
			}
			return nil
		},
	}
}

func sha256File(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extractBinaryFromArchive 按扩展名分派到 tar.gz 或 zip 解包，仅提取与
// 二进制名完全相同的常规文件条目（GoReleaser 归档不含目录层级），写出为
// 可执行的暂存文件并返回其路径；条目大小超过 maxBytes 时拒绝。
func extractBinaryFromArchive(archivePath, destDir, binaryName string, maxBytes int64) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractBinaryFromZip(archivePath, destDir, binaryName, maxBytes)
	}
	return extractBinaryFromTarGz(archivePath, destDir, binaryName, maxBytes)
}

func extractBinaryFromTarGz(archivePath, destDir, binaryName string, maxBytes int64) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return "", errNoBinaryEntry
		}

		if err != nil {
			return "", err
		}

		if hdr.Name != binaryName || hdr.Typeflag != tar.TypeReg || !safeArchiveEntry(hdr.Name) {
			continue
		}
		return writeStagedBinary(destDir, tr, maxBytes)
	}
}

func extractBinaryFromZip(archivePath, destDir, binaryName string, maxBytes int64) (string, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = zr.Close() }()

	for _, zf := range zr.File {
		if zf.Name != binaryName || !zf.FileInfo().Mode().IsRegular() || !safeArchiveEntry(zf.Name) {
			continue
		}

		rc, err := zf.Open()
		if err != nil {
			return "", err
		}

		staged, err := writeStagedBinary(destDir, rc, maxBytes)
		_ = rc.Close()
		return staged, err
	}
	return "", errNoBinaryEntry
}

// safeArchiveEntry rejects absolute paths and path traversal before the entry
// is extracted, even though the staged file name never derives from it.
func safeArchiveEntry(name string) bool {
	if path.IsAbs(name) || strings.Contains(name, "\\") {
		return false
	}

	cleaned := path.Clean(name)
	return cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

// writeStagedBinary 把归档条目写出到目标目录内的临时文件并赋予可执行权限；
// 临时文件与目标二进制同目录，保证后续 os.Rename 在同一文件系统上原子完成。
// Sync 确保替换前数据已落盘，避免崩溃在 rename 与落盘之间留下损坏二进制。
func writeStagedBinary(destDir string, r io.Reader, maxBytes int64) (string, error) {
	f, err := os.CreateTemp(destDir, ".ncmctl.update-*")
	if err != nil {
		return "", fmt.Errorf("%w: %v", errCreateStaging, err)
	}

	staged := f.Name()

	written, err := io.Copy(f, io.LimitReader(r, maxBytes+1))
	if err != nil {
		_ = f.Close()
		_ = os.Remove(staged)
		return "", err
	}

	if written > maxBytes {
		_ = f.Close()
		_ = os.Remove(staged)
		return "", errEntryTooLarge
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(staged)
		return "", err
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(staged)
		return "", err
	}

	if err := os.Chmod(staged, 0o755); err != nil { // #nosec G302 -- staged release binary must be executable
		_ = os.Remove(staged)
		return "", err
	}
	return staged, nil
}
