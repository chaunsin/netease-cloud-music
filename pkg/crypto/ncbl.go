// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package crypto

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/big"

	"github.com/klauspost/compress/zstd"
	"golang.org/x/crypto/chacha20"

	cryptorand "crypto/rand"
)

const (
	NCBLMagic           = "NCBL"
	NCBLVersion         = 3
	NCBLHeaderFixedLen  = 70
	NCBLMetaBlockType   = 0x4343
	NCBLDefaultMaxFrame = 0x8000

	ncblKeySize         = chacha20.KeySize
	ncblUUIDSize        = 16
	ncblNonceSize       = chacha20.NonceSize
	ncblMetaHeaderLen   = 4
	ncblMaxMetadataSize = math.MaxUint16 - NCBLHeaderFixedLen - ncblMetaHeaderLen
)

var (
	ncblRSAModulus, _ = new(big.Int).SetString("fd90bd466ff9bc8a3fec2fbcf263b90d5c564879fa5d7aab89b31c1d5cb4139d", 16)
	ncblRSAExponent   = big.NewInt(65537)
)

// NCBLCompression identifies a body compression format accepted by the NCBL
// endpoint. Zstandard matches modern NetEase clients; gzip remains available
// for runtimes compatible with the JavaScript implementation's fallback.
type NCBLCompression uint8

const (
	NCBLCompressionZstandard NCBLCompression = iota
	NCBLCompressionGzip
)

// NCBLOption customizes EncryptNCBL.
type NCBLOption func(*ncblConfig) error

type ncblConfig struct {
	keyA         [ncblKeySize]byte
	hasKeyA      bool
	uuid         [ncblUUIDSize]byte
	hasUUID      bool
	baseSequence *uint32
	maxFrameSize int
	random       io.Reader
	compress     func([]byte) ([]byte, error)
}

// WithNCBLKey sets the 32-byte record key. EncryptNCBL copies the key and, as
// required by the wire format, clamps a first byte greater than 0xa2.
func WithNCBLKey(key []byte) NCBLOption {
	length := len(key)
	if length != ncblKeySize {
		return func(*ncblConfig) error {
			return fmt.Errorf("ncbl: key must be %d bytes, got %d", ncblKeySize, length)
		}
	}
	var value [ncblKeySize]byte
	copy(value[:], key)
	return func(config *ncblConfig) error {
		config.keyA = value
		config.hasKeyA = true
		return nil
	}
}

// WithNCBLUUID sets the 16-byte UUID from which the ChaCha20 nonce and counter
// are derived. A generated UUID uses the RFC 4122 version 4 and variant bits.
func WithNCBLUUID(uuid []byte) NCBLOption {
	length := len(uuid)
	if length != ncblUUIDSize {
		return func(*ncblConfig) error {
			return fmt.Errorf("ncbl: UUID must be %d bytes, got %d", ncblUUIDSize, length)
		}
	}
	var value [ncblUUIDSize]byte
	copy(value[:], uuid)
	return func(config *ncblConfig) error {
		config.uuid = value
		config.hasUUID = true
		return nil
	}
}

// WithNCBLBaseSequence sets the first record frame sequence number. An explicit
// zero is preserved; omit this option to generate a random sequence number.
func WithNCBLBaseSequence(sequence uint32) NCBLOption {
	return func(config *ncblConfig) error {
		sequenceCopy := sequence
		config.baseSequence = &sequenceCopy
		return nil
	}
}

// WithNCBLMaxFrameSize sets the maximum compressed bytes in each record frame.
// Size must be between 1 and math.MaxUint16. Zero is invalid; omit this option
// to use NCBLDefaultMaxFrame.
func WithNCBLMaxFrameSize(size int) NCBLOption {
	return func(config *ncblConfig) error {
		if size < 1 || size > math.MaxUint16 {
			return fmt.Errorf("ncbl: max frame size must be between 1 and %d, got %d", math.MaxUint16, size)
		}
		config.maxFrameSize = size
		return nil
	}
}

// WithNCBLCompression selects the body compression format.
func WithNCBLCompression(compression NCBLCompression) NCBLOption {
	return func(config *ncblConfig) error {
		compress, err := ncblCompressor(compression)
		if err != nil {
			return err
		}
		config.compress = compress
		return nil
	}
}

// EncryptNCBL encodes metadata and a log body using the NetEase NCBL version 3
// wire format.
func EncryptNCBL(meta, body []byte, options ...NCBLOption) ([]byte, error) {
	if len(meta) > ncblMaxMetadataSize {
		return nil, fmt.Errorf("ncbl: metadata is too large: %d bytes, maximum is %d", len(meta), ncblMaxMetadataSize)
	}

	config, err := newNCBLConfig(options)
	if err != nil {
		return nil, err
	}

	keyA, err := config.recordKey()
	if err != nil {
		return nil, err
	}
	keyB := ncblRSAWrap(keyA)

	uuid, err := config.identifier()
	if err != nil {
		return nil, err
	}
	baseSequence, err := config.firstSequence()
	if err != nil {
		return nil, err
	}

	nonce := uuid[:ncblNonceSize]
	counter := binary.LittleEndian.Uint32(uuid[ncblNonceSize:]) >> 2
	metaCipher, err := ncblChaCha20(keyB, counter, nonce, meta)
	if err != nil {
		return nil, fmt.Errorf("ncbl: encrypt metadata: %w", err)
	}
	headerLen := NCBLHeaderFixedLen + ncblMetaHeaderLen + len(metaCipher)

	compressed, err := config.compress(body)
	if err != nil {
		return nil, fmt.Errorf("ncbl: compress body: %w", err)
	}
	frameCount := ncblFrameCount(len(compressed), config.maxFrameSize)
	trailingLen := uint64(len(compressed)) + uint64(frameCount)*6
	if trailingLen > math.MaxUint32 {
		return nil, fmt.Errorf("ncbl: trailing region is too large: %d bytes", trailingLen)
	}
	payloadLen := uint64(headerLen) + trailingLen
	if payloadLen > uint64(int(^uint(0)>>1)) {
		return nil, fmt.Errorf("ncbl: payload is too large: %d bytes", payloadLen)
	}

	payload := make([]byte, int(payloadLen))
	copy(payload, NCBLMagic)
	binary.LittleEndian.PutUint32(payload[4:8], NCBLVersion)
	binary.LittleEndian.PutUint16(payload[8:10], uint16(headerLen))
	copy(payload[10:26], uuid)
	copy(payload[26:58], keyB)
	binary.LittleEndian.PutUint32(payload[58:62], baseSequence)
	binary.LittleEndian.PutUint32(payload[62:66], baseSequence+uint32(frameCount-1))
	binary.LittleEndian.PutUint32(payload[66:70], uint32(trailingLen))

	metaBlock := payload[NCBLHeaderFixedLen:headerLen]
	binary.LittleEndian.PutUint16(metaBlock[0:2], NCBLMetaBlockType)
	binary.LittleEndian.PutUint16(metaBlock[2:4], uint16(len(metaCipher)))
	copy(metaBlock[4:], metaCipher)

	position := headerLen
	for frame := 0; frame < frameCount; frame++ {
		start := frame * config.maxFrameSize
		end := min(start+config.maxFrameSize, len(compressed))
		ciphertext, err := ncblChaCha20(keyA, counter, nonce, compressed[start:end])
		if err != nil {
			return nil, fmt.Errorf("ncbl: encrypt frame %d: %w", frame, err)
		}

		binary.LittleEndian.PutUint16(payload[position:position+2], uint16(len(ciphertext)))
		binary.LittleEndian.PutUint32(payload[position+2:position+6], baseSequence+uint32(frame))
		copy(payload[position+6:], ciphertext)
		position += 6 + len(ciphertext)
	}

	return payload, nil
}

func newNCBLConfig(options []NCBLOption) (*ncblConfig, error) {
	compress, err := ncblCompressor(NCBLCompressionZstandard)
	if err != nil {
		return nil, err
	}
	config := &ncblConfig{
		maxFrameSize: NCBLDefaultMaxFrame,
		random:       cryptorand.Reader,
		compress:     compress,
	}
	for index, option := range options {
		if option == nil {
			return nil, fmt.Errorf("ncbl: option %d is nil", index)
		}
		if err := option(config); err != nil {
			return nil, fmt.Errorf("ncbl: apply option %d: %w", index, err)
		}
	}
	return config, nil
}

func (config *ncblConfig) recordKey() ([]byte, error) {
	key := make([]byte, ncblKeySize)
	if config.hasKeyA {
		copy(key, config.keyA[:])
	} else {
		if _, err := io.ReadFull(config.random, key); err != nil {
			return nil, fmt.Errorf("ncbl: generate key: %w", err)
		}
	}
	if key[0] >= 0xa3 {
		key[0] = 0xa2
	}
	return key, nil
}

func (config *ncblConfig) identifier() ([]byte, error) {
	uuid := make([]byte, ncblUUIDSize)
	if config.hasUUID {
		copy(uuid, config.uuid[:])
		return uuid, nil
	}

	if _, err := io.ReadFull(config.random, uuid); err != nil {
		return nil, fmt.Errorf("ncbl: generate UUID: %w", err)
	}
	uuid[6] = uuid[6]&0x0f | 0x40
	uuid[8] = uuid[8]&0x3f | 0x80
	return uuid, nil
}

func (config *ncblConfig) firstSequence() (uint32, error) {
	if config.baseSequence != nil {
		return *config.baseSequence, nil
	}

	var data [2]byte
	if _, err := io.ReadFull(config.random, data[:]); err != nil {
		return 0, fmt.Errorf("ncbl: generate base sequence: %w", err)
	}
	return uint32(binary.LittleEndian.Uint16(data[:])), nil
}

func ncblFrameCount(compressedSize, maxFrameSize int) int {
	if compressedSize == 0 {
		return 1
	}
	return 1 + (compressedSize-1)/maxFrameSize
}

func ncblChaCha20(key []byte, counter uint32, nonce, plaintext []byte) ([]byte, error) {
	stream, err := chacha20.NewUnauthenticatedCipher(key, nonce)
	if err != nil {
		return nil, err
	}
	stream.SetCounter(counter)

	ciphertext := make([]byte, len(plaintext))
	stream.XORKeyStream(ciphertext, plaintext)
	return ciphertext, nil
}

func ncblRSAWrap(key []byte) []byte {
	value := new(big.Int).SetBytes(key)
	value.Exp(value, ncblRSAExponent, ncblRSAModulus)
	return value.FillBytes(make([]byte, ncblKeySize))
}

func ncblCompressor(compression NCBLCompression) (func([]byte) ([]byte, error), error) {
	switch compression {
	case NCBLCompressionZstandard:
		return func(body []byte) ([]byte, error) {
			writer, err := zstd.NewWriter(
				nil,
				zstd.WithEncoderConcurrency(1),
				zstd.WithEncoderCRC(false),
				zstd.WithEncoderLevel(zstd.SpeedDefault),
				zstd.WithZeroFrames(true),
			)
			if err != nil {
				return nil, err
			}
			defer writer.Close()
			return writer.EncodeAll(body, nil), nil
		}, nil
	case NCBLCompressionGzip:
		return func(body []byte) ([]byte, error) {
			var buffer bytes.Buffer
			writer := gzip.NewWriter(&buffer)
			if _, err := writer.Write(body); err != nil {
				return nil, err
			}
			if err := writer.Close(); err != nil {
				return nil, err
			}
			return buffer.Bytes(), nil
		}, nil
	default:
		return nil, fmt.Errorf("ncbl: unsupported compression %d", compression)
	}
}

func withNCBLRandomSource(random io.Reader) NCBLOption {
	return func(config *ncblConfig) error {
		if random == nil {
			return fmt.Errorf("ncbl: random source is nil")
		}
		config.random = random
		return nil
	}
}

func withNCBLCompressor(compress func([]byte) ([]byte, error)) NCBLOption {
	return func(config *ncblConfig) error {
		if compress == nil {
			return fmt.Errorf("ncbl: compressor is nil")
		}
		config.compress = compress
		return nil
	}
}
