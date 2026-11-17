// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// 云贝「发布图文笔记」(taskId 6566461) 的服务端规则来自 V3 响应的 taskDescription:
// 「发布一条≥20字的图文笔记」。字数不足时动态仍然发布成功并返回正动态 ID, 但服务端不会
// 创建用户任务实例 (todo 里 userTaskId 始终为 0), 任务进度永不推进——所以正文长度必须在
// 写入前就把住, 而不是发完再等状态变化。
const (
	yunBeiNoteMinMessage = 20
	yunBeiNoteMessage    = "今天听到一首很不错的歌，分享给你们，有空一起听听。"
)

func validateYunBeiOpts(opts *YunBeiOpts, status bool) ([]yunBeiTaskID, error) {
	if status {
		if len(opts.Tasks) != 0 || opts.All || opts.Title != "" || opts.Message != "" || opts.Image != "" || opts.Delete {
			return nil, errors.New("status cannot be combined with execution flags")
		}
		return nil, nil
	}

	if opts.All && len(opts.Tasks) != 0 {
		return nil, errors.New("--task and --all are mutually exclusive")
	}

	selected, err := normalizeYunBeiTasks(opts.Tasks, opts.All)
	if err != nil {
		return nil, err
	}

	if len(selected) == 0 {
		return nil, errors.New("no YunBei task selected: specify --task or --all")
	}

	publishSelected := opts.All || slices.Contains(selected, yunBeiPublishNote)
	if !publishSelected && (opts.Title != "" || opts.Message != "" || opts.Image != "" || opts.Delete) {
		return nil, errors.New("--title, --message, --image and --delete require publish-note or --all")
	}

	if publishSelected {
		if err := validateNoteContent(opts.Title, opts.Message, opts.Image, yunBeiNoteMinMessage); err != nil {
			return nil, err
		}

		if err := validateYunBeiImage(opts.Image); err != nil {
			return nil, err
		}
	}
	return selected, nil
}

func normalizeYunBeiTasks(tasks []string, all bool) ([]yunBeiTaskID, error) {
	if all {
		ids := make([]yunBeiTaskID, 0, len(yunBeiRegistry))
		for _, def := range yunBeiRegistry {
			ids = append(ids, def.ID)
		}
		return ids, nil
	}

	valid := make(map[yunBeiTaskID]struct{}, len(yunBeiRegistry))
	for _, def := range yunBeiRegistry {
		valid[def.ID] = struct{}{}
	}

	seen := make(map[yunBeiTaskID]struct{}, len(tasks))

	selected := make([]yunBeiTaskID, 0, len(tasks))
	for _, value := range tasks {
		for part := range strings.SplitSeq(value, ",") {
			id := yunBeiTaskID(strings.TrimSpace(part))
			if id == "" {
				return nil, errors.New("--task contains an empty value")
			}

			if _, ok := valid[id]; !ok {
				return nil, fmt.Errorf("unknown YunBei task %q; valid tasks: %s", id, yunBeiTaskNames())
			}

			if _, ok := seen[id]; ok {
				continue
			}

			seen[id] = struct{}{}
			selected = append(selected, id)
		}
	}
	return selected, nil
}

func yunBeiTaskNames() string {
	names := make([]string, 0, len(yunBeiRegistry))
	for _, def := range yunBeiRegistry {
		names = append(names, string(def.ID))
	}
	return strings.Join(names, ", ")
}

func validateYunBeiImage(path string) error {
	if path == "" {
		return nil
	}

	if err := validateLocalImage(path); err != nil {
		return fmt.Errorf("image: %w", err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("image: %w", err)
	}

	if info.Size() > dailySongCoverLimit {
		return fmt.Errorf("image: size %d exceeds %d bytes", info.Size(), dailySongCoverLimit)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("image: open: %w", err)
	}

	sample := make([]byte, 512)
	n, readErr := file.Read(sample)
	closeErr := file.Close()

	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return fmt.Errorf("image: read: %w", readErr)
	}

	if closeErr != nil {
		return fmt.Errorf("image: close: %w", closeErr)
	}

	detectedExtension, err := sniffImageSuffix(sample[:n])
	if err != nil {
		return fmt.Errorf("image: %w", err)
	}

	pathExtension := strings.ToLower(filepath.Ext(path))
	if detectedExtension == ".jpg" && pathExtension == ".jpeg" {
		return nil
	}

	if pathExtension != detectedExtension {
		return fmt.Errorf("image: extension %q does not match detected type %q", pathExtension, detectedExtension)
	}
	return nil
}
