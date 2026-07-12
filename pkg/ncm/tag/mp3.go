// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package tag

import (
	"fmt"

	"github.com/bogem/id3v2/v2"
)

type Mp3 struct {
	tag      *id3v2.Tag
	encoding id3v2.Encoding
}

func NewMp3(path string, encoding ...id3v2.Encoding) (*Mp3, error) {
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return nil, fmt.Errorf("id3v2.Open: %w", err)
	}
	encode := id3v2.EncodingUTF8
	if len(encoding) > 0 {
		encode = encoding[0]
	}
	// https://github.com/chaunsin/netease-cloud-music/issues/21#issuecomment-2728279324
	// https://github.com/n10v/id3v2/issues/88
	tag.SetDefaultEncoding(encode) //
	return &Mp3{tag: tag, encoding: encode}, nil
}

func (m *Mp3) SetCover(buf []byte, mime string) error {
	m.tag.AddAttachedPicture(id3v2.PictureFrame{
		Encoding:    m.encoding,
		MimeType:    mime,
		PictureType: id3v2.PTFrontCover,
		Description: "Front cover",
		Picture:     buf,
	})
	return nil
}

func (m *Mp3) SetCoverUrl(coverUrl string) error {
	m.tag.AddAttachedPicture(id3v2.PictureFrame{
		Encoding:    m.encoding,
		MimeType:    "-->",
		PictureType: id3v2.PTFrontCover,
		Description: "Front cover",
		Picture:     []byte(coverUrl),
	})
	return nil
}

func (m *Mp3) SetTitle(title string) error {
	if name := m.tag.Title(); name == "" {
		m.tag.SetTitle(title)
	}
	return nil
}

func (m *Mp3) SetAlbum(album string) error {
	if name := m.tag.Album(); name == "" {
		m.tag.SetAlbum(album)
	}
	return nil
}

func (m *Mp3) SetArtist(artists []string) error {
	// multiple artist support
	if frames := m.tag.GetFrames(m.tag.CommonID("Artist")); len(frames) == 0 {
		for _, artist := range artists {
			m.tag.SetArtist(artist)
		}
	}
	return nil
}

func (m *Mp3) SetComment(comment string) error {
	if frames := m.tag.GetFrames(m.tag.CommonID("Comments")); len(frames) == 0 {
		m.tag.AddCommentFrame(id3v2.CommentFrame{
			Encoding:    m.encoding,
			Language:    "XXX", // todo: ?
			Description: "",
			Text:        comment,
		})
	}
	return nil
}

func (m *Mp3) SetLyrics(lyrics string) error {
	if frames := m.tag.GetFrames(m.tag.CommonID("Unsynchronised lyrics/text transcription")); len(frames) == 0 {
		m.tag.AddUnsynchronisedLyricsFrame(id3v2.UnsynchronisedLyricsFrame{
			Encoding:          m.encoding,
			Language:          "zho", // todo: support other language
			ContentDescriptor: "",
			Lyrics:            lyrics,
		})
	}
	return nil
}

func (m *Mp3) Save() error {
	if err := m.tag.Save(); err != nil {
		_ = m.tag.Close()
		return fmt.Errorf("vesion:%v id3v2.Save: %w", m.tag.Version(), err)
	}
	return m.tag.Close()
}
