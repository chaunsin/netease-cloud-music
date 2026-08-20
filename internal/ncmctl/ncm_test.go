// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNCMCommandDocumentsPositionalInputs(t *testing.T) {
	t.Parallel()

	command := NewNCM(&Root{}, nil).Command()
	assert.Equal(t, "ncm <input> [input...]", command.Use)
	assert.Contains(t, command.Long, "Every positional argument is treated as an input path")
	assert.Contains(t, command.Long, "--output")
	assert.Contains(t, command.Example, "ncmctl ncm")
	require.Error(t, command.Args(command, nil))
	require.NoError(t, command.Args(command, []string{"first.ncm", "second.ncm"}))
}

func TestNCMDirectoryDepthErrorExplainsPositionalInputs(t *testing.T) {
	t.Parallel()

	allowed := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(allowed, "a", "b", "c"), 0o750))

	command := NewNCM(&Root{}, nil)
	files, err := command.scanInputs([]string{allowed})
	require.NoError(t, err)
	assert.Empty(t, files)

	tooDeep := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tooDeep, "a", "b", "c", "d"), 0o750))

	_, err = command.scanInputs([]string{allowed, tooDeep})
	require.Error(t, err)
	require.ErrorContains(t, err, `scan input "`+tooDeep+`"`)
	require.ErrorContains(t, err, "maximum supported input directory depth is 3")
	require.ErrorContains(t, err, "all positional arguments are input paths")
	require.ErrorContains(t, err, "pass a destination with --output instead")
}

func TestNCMExecuteRejectsMissingInput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.ncm")
	missing := filepath.Join(dir, "missing.ncm")

	require.NoError(t, os.WriteFile(valid, nil, 0o600))

	command := NewNCM(&Root{}, nil)
	err := command.execute(context.Background(), []string{valid, missing})
	require.EqualError(t, err, fmt.Sprintf("input %q not found", missing))
}
