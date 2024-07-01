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

	"github.com/stretchr/testify/assert"
)

var (
	ncmName = "./testdata/BOE - 822.ncm"
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
	err = png.Encode(outputFile, image)
	assert.NoError(t, err, "编码并写入PNG文件失败")
}

func TestDump(t *testing.T) {
	var name = ncmName + ".mp3"
	_ = os.Remove(name)
	file, err := os.Open(ncmName)
	defer file.Close()
	assert.NoError(t, err)

	data, err := Decode(file)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	assert.NoError(t, os.WriteFile(name, data, 0644))
}

func TestIsNCMFile(t *testing.T) {
	file, err := os.Open(ncmName)
	defer file.Close()
	assert.NoError(t, err)
	assert.NoError(t, IsNCMFile(file), "判断文件是否是NCM文件失败")
}

func TestCover(t *testing.T) {
	var (
		pngFile  = "../../testdata/test_cover.png"
		jpegFile = "../../testdata/test_cover.jpeg"
	)

	_ = os.Remove(pngFile)
	_ = os.Remove(jpegFile)
	file, err := os.Open(ncmName)
	defer file.Close()
	assert.NoError(t, err)

	data, kind, err := DecodeCover(file)
	assert.NoError(t, err)
	switch kind {
	case CoverTypeJpeg:
		writeJPG(t, bytes.NewReader(data), jpegFile)
	case CoverTypePng:
		writePNG(t, bytes.NewReader(data), pngFile)
	default:
		assert.Fail(t, "不支持的图片格式")
	}
}

func TestMeta(t *testing.T) {
	file, err := os.Open(ncmName)
	defer file.Close()
	assert.NoError(t, err)

	data, err := DecodeMeta(file)
	assert.NoError(t, err)
	assert.NotZero(t, data)
	t.Logf("data:%+v\n", data)
}

func TestDecodeKey(t *testing.T) {
	file, err := os.Open(ncmName)
	defer file.Close()
	assert.NoError(t, err)

	key, err := DecodeKey(file)
	assert.NoError(t, err)
	assert.NotZero(t, key)
}

func TestNewReadSeeker(t *testing.T) {
	file, err := os.Open(ncmName)
	defer file.Close()
	assert.NoError(t, err)

	ncm, err := NewReadSeeker(file)
	assert.NoError(t, err)
	assert.NotZero(t, ncm)
}

func TestOpen(t *testing.T) {
	file, err := Open(ncmName)
	assert.NoError(t, err)

	img, kind := file.Cover()
	assert.NotZero(t, img)
	assert.Contains(t, []CoverType{CoverTypeJpeg, CoverTypePng}, kind)
	assert.NotZero(t, file.Metadata())
	assert.NotZero(t, file.Music())
}
