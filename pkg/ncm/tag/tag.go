package tag

import (
	"bytes"
	"errors"
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
	SetCover(buf []byte, mime string) error // set image buffer
	SetCoverUrl(coverUrl string) error
	SetTitle(string) error
	SetAlbum(string) error
	SetArtist([]string) error
	SetComment(string) error
	Save() error // must be called
}

func New(input, format string) (Tagger, error) {
	var (
		tagger Tagger
		err    error
	)
	switch strings.ToLower(format) {
	case audioFormatMp3:
		tagger, err = NewMp3(input)
	case audioFormatFlac:
		tagger, err = NewFlac(input)
	case audioFormatWav:
		tagger, err = NewWAV(input)
	default:
		err = errors.New(fmt.Sprintf("format: %s is not supportted", format))
	}
	return tagger, err
}

func NewFromNCM(ncm *ncm.NCM, input string) error {
	tag, err := New(input, ncm.Metadata().Format)
	if err != nil {
		return err
	}
	img, _ := ncm.Cover()
	if err := SetMetadata(tag, img, ncm.Metadata()); err != nil {
		return fmt.Errorf("SetMetadata: %w", err)
	}
	return nil
}

func SetMetadata(tag Tagger, imgData []byte, meta *ncm.Metadata) error {
	if imgData == nil && meta.AlbumPic != "" {
		coverData, err := fetchUrl(meta.AlbumPic)
		if err != nil {
			log.Printf("[ncm] fetch %s err:%s", meta.AlbumPic, err)
		} else {
			imgData = coverData
		}
	}

	if imgData != nil {
		var mime string
		if ncm.DetectCoverType(imgData) == ncm.CoverTypeUnknown {
			mime = ncm.CoverTypeJpeg.FileType()
		}
		tag.SetCover(imgData, mime)
	}

	if meta.AlbumPic != "" {
		tag.SetCoverUrl(meta.AlbumPic)
	}

	if meta.Name != "" {
		tag.SetTitle(meta.Name)
	}

	if meta.Album != "" {
		tag.SetAlbum(meta.Album)
	}

	if meta.Comment != "" {
		tag.SetComment(meta.Comment)
	}

	var artists = make([]string, 0)
	for _, artist := range meta.Artists {
		artists = append(artists, artist.Name)
	}
	if len(artists) > 0 {
		tag.SetArtist(artists)
	}
	return tag.Save()
}

func fetchUrl(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, bytes.NewBuffer([]byte{}))
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
