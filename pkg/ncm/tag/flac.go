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

package tag

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	"github.com/go-flac/flacpicture/v2"
	"github.com/go-flac/flacvorbis/v2"
	"github.com/go-flac/go-flac/v2"
)

type Flac struct {
	filename string
	file     *os.File
	flac     *flac.File
	comment  *flacvorbis.MetaDataBlockVorbisComment
}

func NewFlac(filename string) (*Flac, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	_flac, err := flac.ParseBytes(bufio.NewReader(file))
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("ParseBytes: %w", err)
	}

	// find the vorbisComment
	var block *flac.MetaDataBlock
	for _, m := range _flac.Meta {
		if m.Type == flac.VorbisComment {
			block = m
			break
		}
	}
	var comment *flacvorbis.MetaDataBlockVorbisComment
	if block != nil {
		comment, err = flacvorbis.ParseFromMetaDataBlock(*block)
		if err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("ParseFromMetaDataBlock: %w", err)
		}
	} else {
		comment = flacvorbis.New()
	}

	f := Flac{
		filename: filename,
		file:     file,
		flac:     _flac,
		comment:  comment,
	}
	return &f, nil
}

func (f *Flac) SetCover(buf []byte, mime string) error {
	picture, err := flacpicture.NewFromImageData(flacpicture.PictureTypeFrontCover, "Front cover", buf, mime)
	if err != nil {
		return err
	}

	data := picture.Marshal()
	f.flac.Meta = append(f.flac.Meta, &data)
	return nil
}

func (f *Flac) SetCoverUrl(coverUrl string) error {
	picture := &flacpicture.MetadataBlockPicture{
		PictureType: flacpicture.PictureTypeFrontCover,
		MIME:        "-->",
		Description: "Front cover",
		ImageData:   []byte(coverUrl),
	}
	data := picture.Marshal()
	f.flac.Meta = append(f.flac.Meta, &data)
	return nil
}

func (f *Flac) addTag(key string, values ...string) error {
	old, err := f.comment.Get(key)
	if err != nil {
		return err
	}
	if len(old) == 0 {
		for _, val := range values {
			if err = f.comment.Add(key, val); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *Flac) SetTitle(title string) error {
	return f.addTag(flacvorbis.FIELD_TITLE, title)
}

func (f *Flac) SetAlbum(album string) error {

	return f.addTag(flacvorbis.FIELD_ALBUM, album)
}

func (f *Flac) SetArtist(artists []string) error {
	return f.addTag(flacvorbis.FIELD_ARTIST, artists...)
}

func (f *Flac) SetComment(comment string) error {
	return f.addTag(flacvorbis.FIELD_DESCRIPTION, comment)
}

func (f *Flac) setVorbisCommentMeta(block *flac.MetaDataBlock) {
	var idx = -1
	for i, m := range f.flac.Meta {
		if m.Type == flac.VorbisComment {
			idx = i
			break
		}
	}
	if idx == -1 {
		f.flac.Meta = append(f.flac.Meta, block)
	} else {
		f.flac.Meta[idx] = block
	}
}

func (f *Flac) Save() error {
	block := f.comment.Marshal()
	f.setVorbisCommentMeta(&block)

	stat, err := f.file.Stat()
	if err != nil {
		return err
	}

	var tmpName = f.filename + "-tmp"
	temp, err := os.OpenFile(tmpName, os.O_RDWR|os.O_CREATE, stat.Mode())
	if err != nil {
		return err
	}
	defer temp.Close()

	// 写入到临时文件中
	if _, err := f.flac.WriteTo(temp); err != nil {
		return fmt.Errorf("WriteTo: %w", err)
	}

	// 关闭打开的源文件
	if err := f.file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		_ = os.Remove(tmpName)
		return err
	}

	// 替换掉源文件
	if err := os.Rename(tmpName, f.filename); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
