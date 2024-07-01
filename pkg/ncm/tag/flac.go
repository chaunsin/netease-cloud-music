package tag

import (
	"github.com/go-flac/flacpicture"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
)

type Flac struct {
	path string
	file *flac.File
	cmts *flacvorbis.MetaDataBlockVorbisComment
}

func NewFlac(path string) (*Flac, error) {
	// already read and closed
	f, err := flac.ParseFile(path)
	if err != nil {
		return nil, err
	}

	// find the vorbisComment
	var block *flac.MetaDataBlock
	for _, m := range f.Meta {
		if m.Type == flac.VorbisComment {
			block = m
			break
		}
	}
	var cmts *flacvorbis.MetaDataBlockVorbisComment
	if block != nil {
		cmts, err = flacvorbis.ParseFromMetaDataBlock(*block)
		if err != nil {
			return nil, err
		}
	} else {
		cmts = flacvorbis.New()
	}

	tagger := Flac{
		path: path,
		file: f,
		cmts: cmts,
	}
	return &tagger, nil
}

func (f *Flac) SetCover(buf []byte, mime string) error {
	picture, err := flacpicture.NewFromImageData(flacpicture.PictureTypeFrontCover, "Front cover", buf, mime)
	if err != nil {
		return err
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
	old, err := f.cmts.Get(key)
	if err != nil {
		return err
	}
	if len(old) == 0 {
		for _, val := range values {
			if err = f.cmts.Add(key, val); err != nil {
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
	block := f.cmts.Marshal()
	f.setVorbisCommentMeta(&block)
	return f.file.Save(f.path)
}
