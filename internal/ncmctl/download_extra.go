package ncmctl

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // register png decoder
	"net/http"
	"strings"

	"github.com/bogem/id3v2/v2"
	"github.com/chaunsin/netease-cloud-music/pkg/ncm"
	"github.com/go-flac/flacpicture/v2"
	"github.com/go-flac/flacvorbis/v2"
	"github.com/go-flac/go-flac/v2"
	_ "golang.org/x/image/webp" // register webp decoder
)

// ensureJpeg 确保图片数据为 JPEG 格式
func ensureJpeg(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	contentType := http.DetectContentType(data)
	if contentType == "image/jpeg" {
		return data, nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image (%s): %w", contentType, err)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), nil
}

// writeID3v2 写入 ID3v2 标签
func writeID3v2(filePath string, meta *ncm.MetadataMusic, coverData []byte) error {
	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
	if err != nil {
		return err
	}
	defer tag.Close()

	tag.SetDefaultEncoding(id3v2.EncodingUTF8)

	tag.SetTitle(meta.Name)
	var artists []string
	for _, ar := range meta.Artists {
		artists = append(artists, ar.Name)
	}
	tag.SetArtist(strings.Join(artists, "/"))
	tag.SetAlbum(meta.Album)

	if meta.Comment != "" {
		uslt := id3v2.UnsynchronisedLyricsFrame{
			Encoding:          id3v2.EncodingUTF8,
			Language:          "zho",
			ContentDescriptor: "",
			Lyrics:            meta.Comment,
		}
		tag.AddUnsynchronisedLyricsFrame(uslt)
	}

	if len(coverData) > 0 {
		jpegData, err := ensureJpeg(coverData)
		if err != nil {
			// log.Warn("writeID3v2: convert cover to jpeg err: %v", err)
		} else {
			pic := id3v2.PictureFrame{
				Encoding:    id3v2.EncodingUTF8,
				MimeType:    "image/jpeg",
				PictureType: id3v2.PTFrontCover,
				Description: "Cover",
				Picture:     jpegData,
			}
			tag.AddAttachedPicture(pic)
		}
	} else {
		// log.Warn("writeID3v2: coverData is empty for %s", meta.Name)
	}

	return tag.Save()
}

// writeFlac 写入 FLAC 标签
func writeFlac(filePath string, meta *ncm.MetadataMusic, coverData []byte) error {
	f, err := flac.ParseFile(filePath)
	if err != nil {
		return err
	}

	var cmts *flacvorbis.MetaDataBlockVorbisComment
	var cmtIdx int = -1

	// 查找现有的 VorbisComment 块
	for i, b := range f.Meta {
		if b.Type == flac.VorbisComment {
			cmts, err = flacvorbis.ParseFromMetaDataBlock(*b)
			if err != nil {
				return err
			}
			cmtIdx = i
			break
		}
	}

	if cmts == nil {
		cmts = flacvorbis.New()
	}

	// 添加新的元数据
	artists := make([]string, 0, len(meta.Artists))
	for _, ar := range meta.Artists {
		artists = append(artists, ar.Name)
	}

	// 移除可能存在的旧标签以避免重复
	// 注意：go-flac/flacvorbis 似乎没有直接的 Remove 方法，但 Add 会追加
	// 如果需要覆盖，可以考虑清空 Comments map，但这里为了安全起见，我们假设是新文件或者覆盖写入

	cmts.Add(flacvorbis.FIELD_TITLE, meta.Name)
	cmts.Add(flacvorbis.FIELD_ARTIST, strings.Join(artists, "/"))
	cmts.Add(flacvorbis.FIELD_ALBUM, meta.Album)
	if meta.Comment != "" {
		cmts.Add("LYRICS", meta.Comment)
	}

	res := cmts.Marshal()

	if cmtIdx >= 0 {
		f.Meta[cmtIdx] = &res
	} else {
		f.Meta = append(f.Meta, &res)
	}

	// Cover Art
	if len(coverData) > 0 {
		// 移除旧的图片块（如果有）
		var newMeta []*flac.MetaDataBlock
		for _, b := range f.Meta {
			if b.Type != flac.Picture {
				newMeta = append(newMeta, b)
			}
		}
		f.Meta = newMeta

		jpegData, err := ensureJpeg(coverData)
		if err != nil {
			// log.Warn("writeFlac: convert cover to jpeg err: %v", err)
		} else {
			picture, err := flacpicture.NewFromImageData(flacpicture.PictureTypeFrontCover, "Front Cover", jpegData, "image/jpeg")
			if err == nil {
				picBlock := picture.Marshal()
				f.Meta = append(f.Meta, &picBlock)
			} else {
				// log.Warn("writeFlac: NewFromImageData err: %v", err)
			}
		}
	} else {
		// log.Warn("writeFlac: coverData is empty for %s", meta.Name)
	}

	return f.Save(filePath)
}
