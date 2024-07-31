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
	"fmt"

	"github.com/go-flac/flacpicture"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
)

type Flac struct {
	path    string
	file    *flac.File
	comment *flacvorbis.MetaDataBlockVorbisComment
}

func NewFlac(path string) (*Flac, error) {
	// already read and closed
	file, err := flac.ParseFile(path)
	if err != nil {
		return nil, err
	}

	// find the vorbisComment
	var block *flac.MetaDataBlock
	for _, m := range file.Meta {
		if m.Type == flac.VorbisComment {
			block = m
			break
		}
	}
	var comment *flacvorbis.MetaDataBlockVorbisComment
	if block != nil {
		comment, err = flacvorbis.ParseFromMetaDataBlock(*block)
		if err != nil {
			return nil, fmt.Errorf("ParseFromMetaDataBlock: %w", err)
		}
	} else {
		comment = flacvorbis.New()
	}

	f := Flac{
		path:    path,
		file:    file,
		comment: comment,
	}
	return &f, nil
}

// SetCover sets the cover image. mime supported: image/jpeg, image/png
func (f *Flac) SetCover(buf []byte, mime string) error {
	picture, err := flacpicture.NewFromImageData(flacpicture.PictureTypeFrontCover, "Front cover", buf, mime)
	if err != nil {
		return fmt.Errorf("NewFromImageData: %w", err)
	}

	data := picture.Marshal()
	f.file.Meta = append(f.file.Meta, &data)
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
	f.file.Meta = append(f.file.Meta, &data)
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
	for i, m := range f.file.Meta {
		if m.Type == flac.VorbisComment {
			idx = i
			break
		}
	}
	if idx == -1 {
		f.file.Meta = append(f.file.Meta, block)
	} else {
		f.file.Meta[idx] = block
	}
}

func (f *Flac) Save() error {
	block := f.comment.Marshal()
	f.setVorbisCommentMeta(&block)
	return f.file.Save(f.path)
}
