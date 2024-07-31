// MIT License
//
// Copyright (c) 2024 chaunsin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

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
)

var (
	ncmFileName = "../testdata/BOE - 822.ncm"
)

func writeJPG(t *testing.T, data io.Reader, dest string) {
	// 解码JPEG文件
	img, err := jpeg.Decode(data)
	assert.NoError(t, err, "解码JPEG文件失败")

	// 创建输出文件
	outputFile, err := os.Create(dest)
	assert.NoError(t, err, "创建输出文件失败")
	defer outputFile.Close()

	// 编码并写入JPEG文件
	err = jpeg.Encode(outputFile, img, &jpeg.Options{Quality: 100})
	assert.NoError(t, err, "编码并写入JPEG文件失败")
}

func writePNG(t *testing.T, data io.Reader, dest string) {
	// 解码PNG文件
	image, err := png.Decode(data)
	assert.NoError(t, err, "解码PNG文件失败")

	// 创建输出文件
	outputFile, err := os.Create(dest)
	assert.NoError(t, err, "创建输出文件失败")
	defer outputFile.Close()

	// 编码并写入PNG文件
	assert.NoError(t, png.Encode(outputFile, image), "编码并写入PNG文件失败")
}

func TestIsNCMFile(t *testing.T) {
	file, err := os.Open(ncmFileName)
	defer file.Close()
	assert.NoError(t, err)
	assert.NoError(t, IsNCMFile(file), "判断文件是否是NCM文件失败")
}

func TestDecodeKey(t *testing.T) {
	file, err := os.Open(ncmFileName)
	defer file.Close()
	assert.NoError(t, err)

	key, err := DecodeKey(file)
	assert.NoError(t, err)
	assert.NotZero(t, key)
}

func TestMeta(t *testing.T) {
	file, err := os.Open(ncmFileName)
	defer file.Close()
	assert.NoError(t, err)

	data, err := DecodeMeta(file)
	assert.NoError(t, err)
	assert.NotZero(t, data)
	t.Logf("data:%+v\n", data)
}

func TestDecodeCoverType(t *testing.T) {
	file, err := os.Open(ncmFileName)
	defer file.Close()
	assert.NoError(t, err)

	kind, err := DecodeCoverType(file)
	assert.Contains(t, []CoverType{CoverTypeJpeg, CoverTypePng}, kind)
}

func TestDecodeCover(t *testing.T) {
	var (
		pngFile  = ncmFileName + ".png"
		jpegFile = ncmFileName + ".jpeg"
	)

	_ = os.Remove(pngFile)
	_ = os.Remove(jpegFile)
	file, err := os.Open(ncmFileName)
	defer file.Close()
	assert.NoError(t, err)

	var data = new(bytes.Buffer)
	assert.NoError(t, DecodeCover(file, data))
	kind, err := DecodeCoverType(file)
	switch kind {
	case CoverTypeJpeg:
		writeJPG(t, data, jpegFile)
	case CoverTypePng:
		writePNG(t, data, pngFile)
	default:
		assert.Fail(t, "不支持的图片格式")
	}
}

func TestDecodeMusic(t *testing.T) {
	var name = ncmFileName + ".mp3"
	_ = os.Remove(name)
	file, err := os.Open(ncmFileName)
	defer file.Close()
	assert.NoError(t, err)

	dest, err := os.Create(name)
	defer dest.Close()
	assert.NoError(t, err)

	assert.NoError(t, DecodeMusic(file, dest))
}

func TestFromReadSeeker(t *testing.T) {
	file, err := os.Open(ncmFileName)
	defer file.Close()
	assert.NoError(t, err)

	ncm, err := FromReadSeeker(file)
	assert.NoError(t, err)
	assert.NotZero(t, ncm)
}

func TestOpen(t *testing.T) {
	var (
		pngFile  = ncmFileName + "_open.png"
		jpegFile = ncmFileName + "_open.jpeg"
		name     = ncmFileName + "_open.mp3"
	)

	_ = os.Remove(pngFile)
	_ = os.Remove(jpegFile)
	_ = os.Remove(name)

	file, err := Open(ncmFileName)
	defer file.Close()
	assert.NoError(t, err)

	// handler music metadata
	assert.NotZero(t, file.Metadata())

	// handler cover image
	var img = new(bytes.Buffer)
	assert.NoError(t, file.DecodeCover(img))
	assert.NotZero(t, img.Bytes())
	ct, err := file.DecodeCoverType()
	assert.NoError(t, err)
	assert.Contains(t, []CoverType{CoverTypeJpeg, CoverTypePng}, ct)
	switch ct {
	case CoverTypeJpeg:
		writeJPG(t, img, jpegFile)
	case CoverTypePng:
		writePNG(t, img, pngFile)
	}

	// handler music
	var data = new(bytes.Buffer)
	dest, err := os.Create(name)
	defer dest.Close()
	assert.NoError(t, err)
	assert.NoError(t, file.DecodeMusic(data))
	assert.NotZero(t, data.Bytes())
	n, err := dest.Write(data.Bytes())
	assert.NoError(t, err)
	assert.Greater(t, n, 0)
}

func TestMusicDetect(t *testing.T) {
	file, err := os.Open(ncmFileName)
	defer file.Close()
	assert.NoError(t, err)

	var data = new(bytes.Buffer)
	assert.NoError(t, DecodeMusic(file, data))

	m, err := tag.ReadFrom(bytes.NewReader(data.Bytes()))
	assert.NoError(t, err)

	t.Logf("format=%v, name=%v, album=%v", m.FileType(), m.Title(), m.Album())
}
