// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPayloadMarshalOmitsZeroResponse(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(Payload{
		Request: Request{Ciphertext: "request-ciphertext"},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"request":{"ciphertext":"request-ciphertext"}}`, string(data))
}

func TestPayloadMarshalIncludesResponse(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(Payload{
		Request:  Request{Ciphertext: "request-ciphertext"},
		Response: Response{Ciphertext: "response-ciphertext"},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{
		"request": {"ciphertext": "request-ciphertext"},
		"response": {"ciphertext": "response-ciphertext"}
	}`, string(data))
}
