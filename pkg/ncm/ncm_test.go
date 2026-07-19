// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncm

import (
	"bytes"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"testing"

	"github.com/dhowden/tag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ncmFileName = "./testdata/BOE - 822.ncm"

// ncmFileName = "../../testdata/ncm/no_metadata_and_cover.ncm"

func closeNCMTestResource(t *testing.T, closer io.Closer) {
	t.Helper()

	if err := closer.Close(); err != nil {
		t.Errorf("close test resource: %v", err)
	}
}

func writeJPG(t *testing.T, data io.Reader, dest string) {
	t.Helper()

	// 解码JPEG文件
	img, err := jpeg.Decode(data)
	require.NoError(t, err, "解码JPEG文件失败")

	// 创建输出文件
	outputFile, err := os.Create(dest)
	require.NoError(t, err, "创建输出文件失败")

	defer outputFile.Close()

	t.Cleanup(func() {
		_ = os.Remove(dest)
	})

	// 编码并写入JPEG文件
	err = jpeg.Encode(outputFile, img, &jpeg.Options{Quality: 100})
	require.NoError(t, err, "编码并写入JPEG文件失败")
}

func writePNG(t *testing.T, data io.Reader, dest string) {
	t.Helper()

	// 解码PNG文件
	image, err := png.Decode(data)
	require.NoError(t, err, "解码PNG文件失败")

	// 创建输出文件
	outputFile, err := os.Create(dest)
	require.NoError(t, err, "创建输出文件失败")

	defer outputFile.Close()

	t.Cleanup(func() {
		_ = os.Remove(dest)
	})

	// 编码并写入PNG文件
	require.NoError(t, png.Encode(outputFile, image), "编码并写入PNG文件失败")
}

func writeImage(t *testing.T, file io.ReadSeeker, data io.Reader, dest string) {
	t.Helper()

	kind, err := DecodeCoverType(file)
	if err != nil {
		t.Fatalf("DecodeCoverType: %s", err)
	}

	switch kind {
	case CoverTypeJpeg:
		dest += ".jpg"
		writeJPG(t, data, dest)
	case CoverTypePng:
		dest += ".png"
		writePNG(t, data, dest)
	case CoverTypeBmp, CoverTypeWebp, CoverTypeGif:
		assert.Fail(t, "不支持的图片格式:", "%+v", kind)
	default:
		assert.Fail(t, "不支持的图片格式:", "%+v", kind)
	}
}

func TestIsNCMFile(t *testing.T) {
	file, err := os.Open(ncmFileName)

	require.NoError(t, err)
	defer closeNCMTestResource(t, file)

	require.NoError(t, IsNCMFile(file), "判断文件是否是NCM文件失败")
}

func TestDecodeKey(t *testing.T) {
	file, err := os.Open(ncmFileName)

	require.NoError(t, err)
	defer closeNCMTestResource(t, file)

	key, err := DecodeKey(file)
	require.NoError(t, err)
	assert.NotZero(t, key)
}

func TestMeta(t *testing.T) {
	file, err := os.Open(ncmFileName)

	require.NoError(t, err)
	defer closeNCMTestResource(t, file)

	data, err := DecodeMeta(file)
	require.NoError(t, err)
	assert.NotZero(t, data)
	t.Logf("data:%+v\n", data)
}

func TestDecodeCoverType(t *testing.T) {
	accepts := []CoverType{
		CoverTypeJpeg,
		CoverTypePng,
		CoverTypeBmp,
		CoverTypeWebp,
		CoverTypeGif,
	}

	file, err := os.Open(ncmFileName)

	require.NoError(t, err)
	defer closeNCMTestResource(t, file)

	kind, err := DecodeCoverType(file)
	require.NoError(t, err)
	assert.Contains(t, accepts, kind)
}

func TestDecodeCover(t *testing.T) {
	file, err := os.Open(ncmFileName)

	require.NoError(t, err)
	defer closeNCMTestResource(t, file)

	data := new(bytes.Buffer)
	require.NoError(t, DecodeCover(file, data))
	writeImage(t, file, data, ncmFileName)
}

func TestDecodeMusic(t *testing.T) {
	name := ncmFileName + ".mp3"

	t.Cleanup(func() {
		_ = os.Remove(name)
	})

	file, err := os.Open(ncmFileName)

	require.NoError(t, err)
	defer closeNCMTestResource(t, file)

	dest, err := os.Create(name)

	require.NoError(t, err)
	defer closeNCMTestResource(t, dest)

	require.NoError(t, DecodeMusic(file, dest))
}

func TestFromReadSeeker(t *testing.T) {
	file, err := os.Open(ncmFileName)

	require.NoError(t, err)
	defer closeNCMTestResource(t, file)

	ncm, err := FromReadSeeker(file)
	require.NoError(t, err)
	assert.NotZero(t, ncm)
}

func TestOpen(t *testing.T) {
	name := ncmFileName + "_open.mp3"

	t.Cleanup(func() {
		_ = os.Remove(name)
	})

	file, err := Open(ncmFileName)
	if err != nil {
		t.Fatalf("Open: %s", err)
	}
	defer func() {
		require.NoError(t, file.Close())
	}()

	// handler music metadata
	assert.NotZero(t, file.Metadata())

	// handler cover image
	img := new(bytes.Buffer)
	if decodeErr := file.DecodeCover(img); decodeErr != nil {
		t.Fatalf("DecodeCover: %s", decodeErr)
	}

	if img.Len() <= 0 {
		t.Logf("convert len: %v data: %v\n", img.Len(), img.String())
	}

	ct, err := file.DecodeCoverType()
	if err != nil {
		t.Fatalf("DecodeCoverType: %s", err)
	}

	switch ct {
	case CoverTypeJpeg:
		writeJPG(t, img, ncmFileName+"_open.jpeg")
	case CoverTypePng:
		writePNG(t, img, ncmFileName+"_open.png")
	case CoverTypeWebp, CoverTypeGif, CoverTypeBmp:
		t.Logf("不支持的图片格式:%v \n", ct)

		if img.Len() > 0 {
			assert.Fail(t, "不支持的图片格式", "%+v", ct)
		}
	default:
		t.Logf("不支持的图片格式:%v \n", ct)

		if img.Len() > 0 {
			assert.Fail(t, "不支持的图片格式", "%+v", ct)
		}
	}

	// handler music
	data := new(bytes.Buffer)

	dest, err := os.Create(name)

	require.NoError(t, err)
	defer closeNCMTestResource(t, dest)

	require.NoError(t, file.DecodeMusic(data))
	assert.NotZero(t, data.Bytes())
	n, err := dest.Write(data.Bytes())
	require.NoError(t, err)
	assert.Positive(t, n)
}

func TestMusicDetect(t *testing.T) {
	file, err := os.Open(ncmFileName)

	require.NoError(t, err)
	defer closeNCMTestResource(t, file)

	data := new(bytes.Buffer)
	require.NoError(t, DecodeMusic(file, data))

	m, err := tag.ReadFrom(bytes.NewReader(data.Bytes()))
	require.NoError(t, err)

	t.Logf("format=%v, name=%v, album=%v", m.FileType(), m.Title(), m.Album())
}
