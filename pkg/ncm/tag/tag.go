// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package tag

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/chaunsin/netease-cloud-music/pkg/ncm"
)

const (
	audioFormatMp3  = "mp3"
	audioFormatFlac = "flac"
	audioFormatWav  = "wav"
)

// Tagger interface for both mp3 and flac
type Tagger interface {
	SetCover(buf []byte, mime string) error
	SetCoverUrl(coverUrl string) error
	SetTitle(string) error
	SetAlbum(string) error
	SetArtist([]string) error
	SetComment(string) error
	SetLyrics(string) error
	Save() error // must be called
}

func New(filename, format string) (Tagger, error) {
	var (
		tagger Tagger
		err    error
	)
	switch strings.ToLower(format) {
	case audioFormatMp3:
		tagger, err = NewMp3(filename)
	case audioFormatFlac:
		tagger, err = NewFlac(filename)
	case audioFormatWav:
		// tagger, err = NewWAV(filename)
		fallthrough
	default:
		err = fmt.Errorf("format: %s is not supportted", format)
	}
	return tagger, err
}

func NewFromNCM(n *ncm.NCM, filename string) error {
	mata := n.Metadata()
	if mata == nil {
		return fmt.Errorf("ncm.Metadata() is nil")
	}
	var data *ncm.MetadataMusic
	switch mata.GetType() {
	case ncm.MetadataTypeMusic:
		data = mata.GetMusic()
	case ncm.MetadataTypeDJ:
		data = &mata.GetDJ().MainMusic
	default:
		return fmt.Errorf("cover type %s is not supportted", mata.GetType())
	}

	tag, err := New(filename, data.Format)
	if err != nil {
		return err
	}

	img := new(bytes.Buffer)
	if err := n.DecodeCover(img); err != nil {
		return fmt.Errorf("DecodeCover: %w", err)
	}

	if err := SetMetadata(tag, img.Bytes(), data); err != nil {
		return fmt.Errorf("SetMetadata: %w", err)
	}
	return nil
}

func SetMetadata(tag Tagger, imgData []byte, meta *ncm.MetadataMusic) error {
	if imgData == nil && meta.AlbumPic != "" {
		coverData, err := fetchUrl(meta.AlbumPic)
		if err != nil {
			log.Printf("[ncm] fetch %s err:%s", meta.AlbumPic, err)
		} else {
			imgData = coverData
		}
	}

	if len(imgData) > 0 {
		mime := ncm.DetectCoverType(imgData).MIME()
		if err := tag.SetCover(imgData, mime); err != nil {
			return fmt.Errorf("SetCover(%v): %w", mime, err)
		}
	}

	if meta.AlbumPic != "" {
		if err := tag.SetCoverUrl(meta.AlbumPic); err != nil {
			return fmt.Errorf("SetCoverUrl: %w", err)
		}
	}

	if meta.Name != "" {
		if err := tag.SetTitle(meta.Name); err != nil {
			return fmt.Errorf("SetTitle: %w", err)
		}
	}

	if meta.Album != "" {
		if err := tag.SetAlbum(meta.Album); err != nil {
			return fmt.Errorf("SetAlbum: %w", err)
		}
	}

	if meta.Comment != "" {
		if err := tag.SetComment(meta.Comment); err != nil {
			return fmt.Errorf("SetComment: %w", err)
		}
	}

	artists := make([]string, 0)
	for _, artist := range meta.Artists {
		artists = append(artists, artist.Name)
	}
	if len(artists) > 0 {
		if err := tag.SetArtist(artists); err != nil {
			return fmt.Errorf("SetArtist: %w", err)
		}
	}
	return tag.Save()
}

func fetchUrl(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	client := http.Client{
		Timeout: 30 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download album pic: remote returned %d", res.StatusCode)
	}
	defer res.Body.Close()
	return io.ReadAll(res.Body)
}
