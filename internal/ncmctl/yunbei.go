// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/eapi"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

type YunBeiOpts struct {
	Tasks   []string
	All     bool
	Title   string
	Message string
	Image   string
	Delete  bool
}

type YunBei struct {
	root     *Root
	cmd      *cobra.Command
	opts     YunBeiOpts
	l        *log.Logger
	status   bool
	selected []yunBeiTaskID
	image    os.FileInfo
}

func NewYunBei(root *Root, l *log.Logger) *YunBei {
	c := &YunBei{root: root, l: l}
	c.cmd = &cobra.Command{
		Use:   "yunbei [status]",
		Short: "Inspect or run selected YunBei tasks",
		Long: "Inspect YunBei balance, sign-in and task state, or explicitly run selected tasks. " +
			"Execution can publish public notes, interact with comments, artists, playlists and songs, " +
			"share resources, and consume network traffic for long-running playback. Login is required.",
		Example: "  ncmctl yunbei status\n" +
			"  ncmctl yunbei --task sign --task claim\n" +
			"  ncmctl yunbei --task reserve,listen-indie\n" +
			"  ncmctl yunbei --all --delete",
		Args: yunBeiArgs,
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		c.status = len(args) == 1
		return c.execute(cmd.Context())
	}
	return c
}

func (c *YunBei) Command() *cobra.Command {
	return c.cmd
}

func (c *YunBei) addFlags() {
	c.cmd.Flags().StringSliceVarP(&c.opts.Tasks, "task", "t", nil, "tasks to run; comma-separated or repeated")
	c.cmd.Flags().BoolVar(&c.opts.All, "all", false, "run every supported YunBei task, including account-changing and long-running actions")
	c.cmd.Flags().StringVar(&c.opts.Title, "title", "", "title override for the public YunBei note")
	c.cmd.Flags().StringVar(&c.opts.Message, "message", "", "message override for the public YunBei note; at least 20 Unicode characters")
	c.cmd.Flags().StringVar(&c.opts.Image, "image", "", "local JPEG, PNG, GIF or WebP image for the public YunBei note; up to 20 MiB")
	c.cmd.Flags().BoolVar(&c.opts.Delete, "delete", false, "delete only the public note created and confirmed by this run")
}

func yunBeiArgs(_ *cobra.Command, args []string) error {
	if len(args) == 0 || len(args) == 1 && args[0] == "status" {
		return nil
	}
	return errors.New("accepts no argument or the single argument status")
}

func (c *YunBei) validate() error {
	if c.status {
		for _, name := range []string{"task", "all", "title", "message", "image", "delete"} {
			if c.cmd.Flags().Changed(name) {
				return fmt.Errorf("status cannot be combined with --%s", name)
			}
		}
	}

	selected, err := validateYunBeiOpts(&c.opts, c.status)
	if err != nil {
		return err
	}

	if c.opts.Image != "" {
		info, statErr := os.Lstat(c.opts.Image)
		if statErr != nil {
			return fmt.Errorf("image: %w", statErr)
		}

		if c.image == nil {
			c.image = info
		} else if !os.SameFile(c.image, info) || c.image.Size() != info.Size() || !c.image.ModTime().Equal(info.ModTime()) {
			return errors.New("image changed after initial validation")
		}
	}

	c.selected = selected
	return nil
}

// networkTimeout 限定单次拉取封面与音频采样的时长。测试构造的命令没有初始化配置,
// 因此回退到一分钟内置默认值, 而不是让边界校验拿到零超时立即失败。
func (c *YunBei) networkTimeout() time.Duration {
	if c.root != nil && c.root.Cfg != nil && c.root.Cfg.Network != nil && c.root.Cfg.Network.Timeout > 0 {
		return c.root.Cfg.Network.Timeout
	}
	return time.Minute
}

func (c *YunBei) execute(ctx context.Context) error {
	if err := c.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	runCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	cli, err := api.NewClient(c.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}

	var (
		runtime *yunBeiRuntime
		runErr  error
	)

	defer func() {
		withTimeout := context.WithTimeout
		if runtime != nil {
			withTimeout = runtime.withTimeout
		}

		closeCtx, cancel := withTimeout(context.WithoutCancel(runCtx), yunBeiCleanupTimeout)
		defer cancel()

		closeAPIClient(closeCtx, cli, c.l)
	}()

	runtime, runErr = c.runCore(runCtx, cli)

	withTimeout := context.WithTimeout
	if runtime != nil {
		withTimeout = runtime.withTimeout
	}

	closeCtx, cancel := withTimeout(context.WithoutCancel(runCtx), yunBeiCleanupTimeout)
	defer cancel()

	var cleanupErr error
	if runtime != nil && !c.status {
		cleanupErr = runtime.cleanup(closeCtx)
		c.printSummary(runtime)
	}

	return errors.Join(runErr, cleanupErr)
}

func (c *YunBei) run(ctx context.Context, cli *api.Client) error {
	runtime, runErr := c.runCore(ctx, cli)
	if runtime == nil || c.status {
		return runErr
	}

	cleanupCtx, cancel := runtime.withTimeout(context.WithoutCancel(ctx), yunBeiCleanupTimeout)
	defer cancel()

	cleanupErr := runtime.cleanup(cleanupCtx)
	c.printSummary(runtime)
	return errors.Join(runErr, cleanupErr)
}

func (c *YunBei) runCore(ctx context.Context, cli *api.Client) (*yunBeiRuntime, error) {
	w := weapi.New(cli)

	user, err := w.GetUserInfo(ctx, &weapi.GetUserInfoReq{})
	if err != nil {
		return nil, fmt.Errorf("GetUserInfo: %w", err)
	}

	if user.Code != 200 {
		return nil, fmt.Errorf("GetUserInfo: code=%d message=%q", user.Code, user.Message)
	}

	if user.Account == nil {
		return nil, errors.New("GetUserInfo: account is missing")
	}

	if user.Profile == nil {
		return nil, errors.New("GetUserInfo: profile is missing")
	}

	if user.Profile.UserId <= 0 {
		return nil, errors.New("GetUserInfo: profile user ID is invalid")
	}

	runtime := newYunBeiRuntime(c, w, eapi.New(cli), user.Profile.UserId)
	runtime.vipType = user.Account.VipType
	c.cmd.Printf("\n云贝任务\n\n  账号: %s (%d)\n", user.Profile.Nickname, user.Profile.UserId)

	if c.status {
		return runtime, c.printStatus(ctx, runtime)
	}

	c.printExecutionPlan()

	selected := make(map[yunBeiTaskID]struct{}, len(c.selected))
	for _, id := range c.selected {
		selected[id] = struct{}{}
	}

	errs := make([]error, 0, len(c.selected))

	errs = append(errs, c.runStandaloneStage(ctx, runtime, selected, stageAccount)...)

	errs = append(errs, c.runYunBeiTaskRounds(ctx, runtime, &yunBeiRoundPlan{
		registry: yunBeiRegistry,
		selected: selected,
		load:     runtime.loadTaskSnapshot,
	})...)

	errs = append(errs, c.runStandaloneStage(ctx, runtime, selected, stageClaim)...)
	return runtime, errors.Join(errs...)
}
