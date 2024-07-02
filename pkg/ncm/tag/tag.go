package tag

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/chaunsin/netease-cloud-music/pkg/ncm"
)

type Format string

const (
	FormatMp3  Format = "mp3"
	FormatFlac Format = "flac"
	FormatWav  Format = "wav"
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

func New(input string, format Format) (Tagger, error) {
	var (
		tagger Tagger
		err    error
	)
	switch format {
	case FormatMp3:
		tagger, err = NewMp3(input)
	case FormatFlac:
		tagger, err = NewFlac(input)
	case FormatWav:
		tagger, err = NewWAV(input)
	default:
		err = errors.New(fmt.Sprintf("format: %s is not supportted", format))
	}
	return tagger, err
}

func NewFromNCM(_ncm *ncm.NCM, input string) error {
	var (
		meta     = _ncm.Metadata()
		metadata *ncm.MetadataMusic
		format   string
	)
	switch meta.GetType() {
	case ncm.MetadataTypeMusic:
		format = meta.GetMusic().Format
		metadata = meta.GetMusic()
	case ncm.MetadataTypeDJ:
		format = meta.GetDJ().MainMusic.Format
		metadata = &meta.GetDJ().MainMusic
	}

	tag, err := New(input, Format(format))
	if err != nil {
		return err
	}
	img, _ := _ncm.Cover()
	if err := SetMetadata(tag, img, metadata); err != nil {
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
