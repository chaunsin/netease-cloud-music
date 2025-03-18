package tag

import (
	"io"
	"os"
	"testing"

	"github.com/bogem/id3v2/v2"
	"github.com/stretchr/testify/assert"
)

func TestMp3_Save(t *testing.T) {
	type args struct {
		path     string
		dest     string
		encoding id3v2.Encoding
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "EncodingUTF8",
			args: args{
				path:     "../testdata/not_supported_by_encoding.mp3",
				dest:     "../testdata/not_supported_by_encoding_utf8.mp3",
				encoding: id3v2.EncodingUTF8,
			},
			wantErr: false,
		},
		{
			name: "EncodingISO",
			args: args{
				path:     "../testdata/not_supported_by_encoding.mp3",
				dest:     "../testdata/not_supported_by_encoding_iso.mp3",
				encoding: id3v2.EncodingISO,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := os.Open(tt.args.path)
			if err != nil {
				t.Fatalf("os.Open() error = %v", err)
				return
			}
			t.Cleanup(func() {
				src.Close()
			})

			dest, err := os.Create(tt.args.dest)
			assert.NoError(t, err)
			t.Cleanup(func() {
				dest.Close()
				os.Remove(tt.args.dest)
			})

			if _, err = io.Copy(dest, src); err != nil {
				t.Fatalf("io.Copy() error = %v", err)
			}

			m, err := NewMp3(tt.args.dest, tt.args.encoding)
			if err != nil {
				t.Fatalf("NewMp3() error = %v", err)
				return
			}

			_ = m.SetComment("评论")
			_ = m.SetTitle("标题")
			_ = m.SetArtist([]string{"歌手1", "歌手2"})
			_ = m.SetAlbum("专辑")
			_ = m.SetCoverUrl("https://www.baidu.com/img/bd_logo1.png")
			_ = m.SetCover([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, "image/jpeg")

			if err := m.Save(); (err != nil) != tt.wantErr {
				t.Errorf("Save() encoding=%+v error = %v, wantErr %v", tt.args.encoding, err, tt.wantErr)
			}
		})
	}
}
