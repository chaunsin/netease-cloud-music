// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package crypto

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"math/big"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNCBLChaCha20RFC8439Vector(t *testing.T) {
	key := decodeHex(t, "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	nonce := decodeHex(t, "000000090000004a00000000")
	want := decodeHex(t, "10f1e7e4d13b5915500fdd1fa32071c4c7d1f4c733c068030422aa9ac3d46c4e"+
		"d2826446079faa0914c2d705d98b02a2b5129cd1de164eb9cbd083e8a2503c4e")

	got, err := ncblChaCha20(key, 1, nonce, make([]byte, 64))
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestNCBLChaCha20JavaScriptVectorCrossesBlockBoundary(t *testing.T) {
	key := decodeHex(t, "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	nonce := decodeHex(t, "000000090000004a00000000")
	plaintext := make([]byte, 80)
	for index := range plaintext {
		plaintext[index] = byte(index)
	}
	// Generated with upstream util/ncbl.js at commit 63f669d7 and Node v24.3.0.
	want := decodeHex(t, "10f0e5e7d53e5f125806d714af2d7fcbd7c0e6d427d57e141c3bb081dfc97251"+
		"f2a3466523ba8c2e3cebfd2ef5a62c8d8523aee2ea23788ef3e9b9d39e6d0271"+
		"4ac9c1347d92f909b085e6fba666f799")

	got, err := ncblChaCha20(key, 1, nonce, plaintext)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestNCBLRSAWrapJavaScriptVector(t *testing.T) {
	key := decodeHex(t, "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	want := decodeHex(t, "dda20437ce173c34273cb03bffb85db8f3ce53bc2f1334a752303d26890094af")

	assert.Equal(t, want, ncblRSAWrap(key))
}

func TestEncryptNCBLMatchesJavaScriptGolden(t *testing.T) {
	key := decodeHex(t, "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	uuid := decodeHex(t, "000102030405060708090a0b0c0d0e0f")
	want := decodeHex(t, "4e43424c030000005700000102030405060708090a0b0c0d0e0fdda20437ce173c34"+
		"273cb03bffb85db8f3ce53bc2f1334a752303d26890094af341200003512000017000000"+
		"43430d0002c79995a986014d2e06be7d5d08003412000060305e9fa63e3b360300351200004a1938")

	got, err := EncryptNCBL(
		[]byte(`{"meta":"ok"}`),
		[]byte("hello NCBL\n"),
		WithNCBLKey(key),
		WithNCBLUUID(uuid),
		WithNCBLBaseSequence(0x1234),
		WithNCBLMaxFrameSize(8),
		withNCBLCompressor(func(body []byte) ([]byte, error) {
			return bytes.Clone(body), nil
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestEncryptNCBLCompressionRoundTrip(t *testing.T) {
	meta := []byte(`{"MUSIC_U":"token","os":"pc"}`)
	body := make([]byte, 2048)
	for index := range body {
		body[index] = byte(index*31 + 7)
	}
	key := decodeHex(t, "a50102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	uuid := decodeHex(t, "00112233445566778899aabbccddeeff")

	tests := []struct {
		name       string
		options    []NCBLOption
		magic      []byte
		decompress func(*testing.T, []byte) []byte
	}{
		{
			name:       "default zstandard",
			magic:      []byte{0x28, 0xb5, 0x2f, 0xfd},
			decompress: decompressNCBLZstandard,
		},
		{
			name:       "gzip fallback",
			options:    []NCBLOption{WithNCBLCompression(NCBLCompressionGzip)},
			magic:      []byte{0x1f, 0x8b},
			decompress: decompressNCBLGzip,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := []NCBLOption{
				WithNCBLKey(key),
				WithNCBLUUID(uuid),
				WithNCBLBaseSequence(0xfffffffe),
				WithNCBLMaxFrameSize(17),
			}
			options = append(options, test.options...)

			payload, err := EncryptNCBL(meta, body, options...)
			require.NoError(t, err)
			decoded := decodeNCBL(t, payload)

			assert.Equal(t, meta, decoded.meta)
			assert.Equal(t, uuid, decoded.uuid)
			assert.Equal(t, byte(0xa2), decoded.keyA[0])
			assert.Equal(t, key[1:], decoded.keyA[1:])
			assert.Equal(t, uint32(0xfffffffe), decoded.firstSequence)
			assert.Equal(t, decoded.firstSequence+uint32(len(decoded.sequences)-1), decoded.lastSequence)
			for index, sequence := range decoded.sequences {
				assert.Equal(t, decoded.firstSequence+uint32(index), sequence)
			}
			require.True(t, bytes.HasPrefix(decoded.compressed, test.magic))
			assert.Equal(t, body, test.decompress(t, decoded.compressed))
		})
	}

	assert.Equal(t, byte(0xa5), key[0], "EncryptNCBL must not clamp the caller's slice")
}

func TestEncryptNCBLEmptyBodyIsAValidZstandardFrame(t *testing.T) {
	payload, err := EncryptNCBL(
		nil,
		nil,
		WithNCBLKey(make([]byte, ncblKeySize)),
		WithNCBLUUID(make([]byte, ncblUUIDSize)),
		WithNCBLBaseSequence(0),
	)
	require.NoError(t, err)

	decoded := decodeNCBL(t, payload)
	assert.Equal(t, decodeHex(t, "28b52ffd2000010000"), decoded.compressed)
	assert.Len(t, decoded.sequences, 1)
	assert.Empty(t, decompressNCBLZstandard(t, decoded.compressed))
}

func TestEncryptNCBLGeneratesAndClampsProtocolValues(t *testing.T) {
	random := append(bytes.Repeat([]byte{0xff}, ncblKeySize), bytes.Repeat([]byte{0xff}, ncblUUIDSize)...)
	random = append(random, 0x34, 0x12)

	payload, err := EncryptNCBL(
		nil,
		nil,
		withNCBLRandomSource(bytes.NewReader(random)),
		withNCBLCompressor(func(body []byte) ([]byte, error) { return nil, nil }),
	)
	require.NoError(t, err)

	decoded := decodeNCBL(t, payload)
	assert.Equal(t, byte(0xa2), decoded.keyA[0])
	assert.Equal(t, byte(0x4f), decoded.uuid[6])
	assert.Equal(t, byte(0xbf), decoded.uuid[8])
	assert.Equal(t, uint32(0x1234), decoded.firstSequence)
	assert.Len(t, decoded.sequences, 1)
}

func TestNCBLOptionsCopyCallerInputs(t *testing.T) {
	key := decodeHex(t, "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	uuid := decodeHex(t, "00112233445566778899aabbccddeeff")
	keyOption := WithNCBLKey(key)
	uuidOption := WithNCBLUUID(uuid)
	key[0] = 0xff
	uuid[0] = 0xff

	payload, err := EncryptNCBL(
		nil,
		nil,
		keyOption,
		uuidOption,
		WithNCBLBaseSequence(1),
		withNCBLCompressor(func(body []byte) ([]byte, error) { return nil, nil }),
	)
	require.NoError(t, err)

	decoded := decodeNCBL(t, payload)
	assert.Equal(t, byte(0x00), decoded.keyA[0])
	assert.Equal(t, byte(0x00), decoded.uuid[0])
}

func TestNCBLExplicitZeroOptionSemantics(t *testing.T) {
	payload, err := EncryptNCBL(
		nil,
		nil,
		WithNCBLKey(make([]byte, ncblKeySize)),
		WithNCBLUUID(make([]byte, ncblUUIDSize)),
		WithNCBLBaseSequence(0),
		withNCBLRandomSource(bytes.NewReader(nil)),
		withNCBLCompressor(func(body []byte) ([]byte, error) { return nil, nil }),
	)
	require.NoError(t, err)
	decoded := decodeNCBL(t, payload)
	assert.Equal(t, uint32(0), decoded.firstSequence)
	assert.Equal(t, uint32(0), decoded.lastSequence)
	assert.Equal(t, []uint32{0}, decoded.sequences)

	_, err = EncryptNCBL(nil, nil, WithNCBLMaxFrameSize(0))
	assert.ErrorContains(t, err, "max frame size must be between 1 and 65535, got 0")
}

func TestNCBLOptionsCanBeReusedConcurrently(t *testing.T) {
	options := []NCBLOption{
		WithNCBLKey(decodeHex(t, "a50102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")),
		WithNCBLUUID(decodeHex(t, "00112233445566778899aabbccddeeff")),
		WithNCBLBaseSequence(7),
		withNCBLCompressor(func(body []byte) ([]byte, error) { return bytes.Clone(body), nil }),
	}
	want, err := EncryptNCBL([]byte("meta"), []byte("body"), options...)
	require.NoError(t, err)

	type result struct {
		payload []byte
		err     error
	}
	results := make(chan result, 16)
	for range cap(results) {
		go func() {
			payload, err := EncryptNCBL([]byte("meta"), []byte("body"), options...)
			results <- result{payload: payload, err: err}
		}()
	}
	for range cap(results) {
		got := <-results
		require.NoError(t, got.err)
		assert.Equal(t, want, got.payload)
	}
}

func TestEncryptNCBLRejectsInvalidOptions(t *testing.T) {
	tests := []struct {
		name   string
		option NCBLOption
	}{
		{name: "short key", option: WithNCBLKey(make([]byte, ncblKeySize-1))},
		{name: "long key", option: WithNCBLKey(make([]byte, ncblKeySize+1))},
		{name: "short UUID", option: WithNCBLUUID(make([]byte, ncblUUIDSize-1))},
		{name: "long UUID", option: WithNCBLUUID(make([]byte, ncblUUIDSize+1))},
		{name: "oversized frame", option: WithNCBLMaxFrameSize(1 << 16)},
		{name: "unknown compression", option: WithNCBLCompression(NCBLCompression(100))},
		{name: "nil random source", option: withNCBLRandomSource(nil)},
		{name: "nil compressor", option: withNCBLCompressor(nil)},
		{name: "nil option", option: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := EncryptNCBL(nil, nil, test.option)
			assert.Error(t, err)
		})
	}
}

func TestEncryptNCBLMetadataLimit(t *testing.T) {
	t.Run("maximum size is accepted", func(t *testing.T) {
		payload, err := EncryptNCBL(
			make([]byte, ncblMaxMetadataSize),
			nil,
			WithNCBLKey(make([]byte, ncblKeySize)),
			WithNCBLUUID(make([]byte, ncblUUIDSize)),
			WithNCBLBaseSequence(0),
			withNCBLCompressor(func(body []byte) ([]byte, error) { return nil, nil }),
		)
		require.NoError(t, err)
		assert.Equal(t, uint16(0xffff), binary.LittleEndian.Uint16(payload[8:10]))
	})

	t.Run("oversized input is rejected before options", func(t *testing.T) {
		optionApplied := false
		probe := NCBLOption(func(*ncblConfig) error {
			optionApplied = true
			return nil
		})

		_, err := EncryptNCBL(make([]byte, ncblMaxMetadataSize+1), nil, probe)
		assert.ErrorContains(t, err, "65462 bytes")
		assert.ErrorContains(t, err, "maximum is 65461")
		assert.False(t, optionApplied)
	})
}

func TestEncryptNCBLPropagatesRandomAndCompressionErrors(t *testing.T) {
	_, err := EncryptNCBL(nil, nil, withNCBLRandomSource(bytes.NewReader(nil)))
	assert.ErrorContains(t, err, "generate key")

	want := errors.New("compression failed")
	_, err = EncryptNCBL(
		nil,
		nil,
		WithNCBLKey(make([]byte, ncblKeySize)),
		WithNCBLUUID(make([]byte, ncblUUIDSize)),
		WithNCBLBaseSequence(0),
		withNCBLCompressor(func(body []byte) ([]byte, error) { return nil, want }),
	)
	assert.ErrorIs(t, err, want)
}

type decodedNCBLPayload struct {
	meta          []byte
	compressed    []byte
	uuid          []byte
	keyA          []byte
	firstSequence uint32
	lastSequence  uint32
	sequences     []uint32
}

func decodeNCBL(t *testing.T, payload []byte) decodedNCBLPayload {
	t.Helper()
	require.GreaterOrEqual(t, len(payload), NCBLHeaderFixedLen)
	assert.Equal(t, NCBLMagic, string(payload[:4]))
	assert.Equal(t, uint32(NCBLVersion), binary.LittleEndian.Uint32(payload[4:8]))

	headerLen := int(binary.LittleEndian.Uint16(payload[8:10]))
	require.GreaterOrEqual(t, headerLen, NCBLHeaderFixedLen)
	require.LessOrEqual(t, headerLen, len(payload))
	uuid := bytes.Clone(payload[10:26])
	keyB := bytes.Clone(payload[26:58])
	firstSequence := binary.LittleEndian.Uint32(payload[58:62])
	lastSequence := binary.LittleEndian.Uint32(payload[62:66])
	trailingLen := int(binary.LittleEndian.Uint32(payload[66:70]))
	assert.Equal(t, len(payload)-headerLen, trailingLen)

	keyA := unwrapNCBLKey(t, keyB)
	nonce := uuid[:ncblNonceSize]
	counter := binary.LittleEndian.Uint32(uuid[ncblNonceSize:]) >> 2

	var meta bytes.Buffer
	for position := NCBLHeaderFixedLen; position < headerLen; {
		require.LessOrEqual(t, position+ncblMetaHeaderLen, headerLen)
		blockType := binary.LittleEndian.Uint16(payload[position : position+2])
		blockLen := int(binary.LittleEndian.Uint16(payload[position+2 : position+4]))
		position += ncblMetaHeaderLen
		require.LessOrEqual(t, position+blockLen, headerLen)
		if blockType == NCBLMetaBlockType {
			plaintext, err := ncblChaCha20(keyB, counter, nonce, payload[position:position+blockLen])
			require.NoError(t, err)
			_, err = meta.Write(plaintext)
			require.NoError(t, err)
		}
		position += blockLen
	}

	var compressed bytes.Buffer
	sequences := make([]uint32, 0)
	for position := headerLen; position < len(payload); {
		require.LessOrEqual(t, position+6, len(payload))
		frameLen := int(binary.LittleEndian.Uint16(payload[position : position+2]))
		sequences = append(sequences, binary.LittleEndian.Uint32(payload[position+2:position+6]))
		position += 6
		require.LessOrEqual(t, position+frameLen, len(payload))
		plaintext, err := ncblChaCha20(keyA, counter, nonce, payload[position:position+frameLen])
		require.NoError(t, err)
		_, err = compressed.Write(plaintext)
		require.NoError(t, err)
		position += frameLen
	}

	return decodedNCBLPayload{
		meta:          meta.Bytes(),
		compressed:    compressed.Bytes(),
		uuid:          uuid,
		keyA:          keyA,
		firstSequence: firstSequence,
		lastSequence:  lastSequence,
		sequences:     sequences,
	}
}

func unwrapNCBLKey(t *testing.T, keyB []byte) []byte {
	t.Helper()
	p, ok := new(big.Int).SetString("337838269511367116547262517807543394287", 10)
	require.True(t, ok)
	q, ok := new(big.Int).SetString("339484579896250424463517790785600633139", 10)
	require.True(t, ok)

	pMinusOne := new(big.Int).Sub(p, big.NewInt(1))
	qMinusOne := new(big.Int).Sub(q, big.NewInt(1))
	phi := new(big.Int).Mul(pMinusOne, qMinusOne)
	privateExponent := new(big.Int).ModInverse(ncblRSAExponent, phi)
	require.NotNil(t, privateExponent)

	modulus := new(big.Int).Mul(p, q)
	assert.Equal(t, 0, modulus.Cmp(ncblRSAModulus))
	key := new(big.Int).SetBytes(keyB)
	key.Exp(key, privateExponent, modulus)
	return key.FillBytes(make([]byte, ncblKeySize))
}

func decompressNCBLZstandard(t *testing.T, compressed []byte) []byte {
	t.Helper()
	reader, err := zstd.NewReader(nil)
	require.NoError(t, err)
	defer reader.Close()

	body, err := reader.DecodeAll(compressed, nil)
	require.NoError(t, err)
	return body
}

func decompressNCBLGzip(t *testing.T, compressed []byte) []byte {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	require.NoError(t, err)
	defer reader.Close()

	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	return body
}

func decodeHex(t *testing.T, value string) []byte {
	t.Helper()
	data, err := hex.DecodeString(value)
	require.NoError(t, err)
	return data
}
