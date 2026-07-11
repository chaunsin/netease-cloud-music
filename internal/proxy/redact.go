// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	redactedValue               = "[REDACTED]"
	unsafeBodyPlaceholderReason = "body omitted: unable to safely redact unstructured data"
	unsafeTextPlaceholder       = "[REDACTED UNSTRUCTURED TEXT]"
	maxJSONDisplayDepth         = 64
	defaultJSONDisplayLimit     = 1 << 20
	maxNestedJSONDisplayBytes   = 4 << 10
)

var (
	errJSONDisplayLimit = errors.New("formatted JSON exceeds the display limit")
	errJSONDepth        = errors.New("JSON nesting exceeds the display depth limit")
	errJSONInputLimit   = errors.New("JSON input exceeds the display limit")

	sensitiveTextKey = `(?:authorization|proxy[-_ ]?authorization|cookie|set[-_ ]?cookie|password|passwd|pwd|credential|credentials|secret|signature|music(?:[_-]?(?:r[_-]?)?[ua]|[_-]?a[_-]?t)|phone|phone[_-]?number|cellphone|mobile|email|imei|imsi|device[-_ ]?(?:id|identifier)|captcha|verification[-_ ]?code|sms[-_ ]?code|[a-z0-9_.-]*(?:token|csrf|secret|access[-_]?key|api[-_]?key|session[-_]?key|sskey)[a-z0-9_.-]*)`
	sensitiveLine    = regexp.MustCompile(`(?im)^(\s*` + sensitiveTextKey + `\s*:\s*).*$`)
	sensitiveInline  = regexp.MustCompile(`(?i)\b(` + sensitiveTextKey + `)\b(\s*[:=]\s*)(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s&;,\r\n]+)`)
	diagnosticURL    = regexp.MustCompile(`(?i)https?://[^\s]+`)
)

type jsonDisplayMeta struct {
	requestEncrypted bool
	rootURL          string
}

type limitedDisplayBuffer struct {
	buffer bytes.Buffer
	limit  int64
}

func newLimitedDisplayBuffer(limit int64, initialSize int) (*limitedDisplayBuffer, error) {
	if limit <= 0 {
		return nil, errJSONDisplayLimit
	}
	buffer := &limitedDisplayBuffer{limit: limit}
	if initialSize > 0 {
		if int64(initialSize) > limit {
			initialSize = int(limit)
		}
		buffer.buffer.Grow(initialSize)
	}
	return buffer, nil
}

func (b *limitedDisplayBuffer) writeString(value string) error {
	if int64(len(value)) > b.limit-int64(b.buffer.Len()) {
		return errJSONDisplayLimit
	}
	b.buffer.WriteString(value)
	return nil
}

func (b *limitedDisplayBuffer) writeByte(value byte) error {
	if int64(b.buffer.Len()) >= b.limit {
		return errJSONDisplayLimit
	}
	b.buffer.WriteByte(value)
	return nil
}

func (b *limitedDisplayBuffer) bytes() []byte {
	return b.buffer.Bytes()
}

func redactHeaders(header http.Header, showSensitive bool) http.Header {
	redacted := make(http.Header, len(header))
	for key, values := range header {
		copied := append([]string(nil), values...)
		if !showSensitive && sensitiveKey(key) {
			for i := range copied {
				copied[i] = redactedValue
			}
		} else if !showSensitive {
			for i := range copied {
				copied[i] = redactJSONString(copied[i], false)
			}
		}
		redacted[key] = copied
	}
	return redacted
}

func redactURL(u *url.URL, showSensitive bool) string {
	if u == nil {
		return ""
	}
	copyURL := *u
	if !showSensitive {
		if copyURL.User != nil {
			copyURL.User = url.User(redactedValue)
		}
		copyURL.RawQuery = redactValues(copyURL.Query(), false).Encode()
	}
	return copyURL.String()
}

func redactValues(values url.Values, showSensitive bool) url.Values {
	result := make(url.Values, len(values))
	for key, entries := range values {
		result[key] = make([]string, len(entries))
		for i, entry := range entries {
			if !showSensitive && sensitiveKey(key) {
				result[key][i] = redactedValue
				continue
			}
			result[key][i] = redactJSONString(entry, showSensitive)
		}
	}
	return result
}

// formatJSON is retained for focused unit tests. Proxy capture paths must use
// formatJSONLimited with the configured display limit.
func formatJSON(data []byte, showSensitive bool) ([]byte, any, error) {
	formatted, _, err := formatJSONLimited(data, showSensitive, defaultJSONDisplayLimit)
	if err != nil {
		return nil, nil, err
	}
	value, err := decodeJSON(data)
	if err != nil {
		return nil, nil, err
	}
	return formatted, value, nil
}

// formatJSONLimited streams JSON tokens into a bounded output buffer. It never
// creates a full pretty-printed copy before enforcing the configured limit.
func formatJSONLimited(data []byte, showSensitive bool, limit int64) ([]byte, jsonDisplayMeta, error) {
	if err := validateJSONForDisplay(data, limit); err != nil {
		return nil, jsonDisplayMeta{}, err
	}
	output, err := newLimitedDisplayBuffer(limit, len(data))
	if err != nil {
		return nil, jsonDisplayMeta{}, err
	}
	formatter := jsonDisplayFormatter{
		decoder:       json.NewDecoder(bytes.NewReader(data)),
		output:        output,
		showSensitive: showSensitive,
	}
	formatter.decoder.UseNumber()
	if err := formatter.writeValue("", 0, false); err != nil {
		return nil, jsonDisplayMeta{}, err
	}
	if _, err := formatter.decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, jsonDisplayMeta{}, errors.New("multiple JSON values")
		}
		return nil, jsonDisplayMeta{}, err
	}
	return output.bytes(), formatter.meta, nil
}

func validateJSONForDisplay(data []byte, limit int64) error {
	if limit <= 0 || int64(len(data)) > limit {
		return errJSONInputLimit
	}
	if !utf8.Valid(data) {
		return errors.New("JSON input is not valid UTF-8")
	}
	return validateJSONDepth(data)
}

func validateJSONDepth(data []byte) error {
	depth := 0
	inString := false
	escaped := false
	for _, value := range data {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch value {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch value {
		case '"':
			inString = true
		case '{', '[':
			depth++
			if depth > maxJSONDisplayDepth {
				return errJSONDepth
			}
		case '}', ']':
			depth--
		}
	}
	return nil
}

func decodeJSON(data []byte) (any, error) {
	if err := validateJSONDepth(data); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("multiple JSON values")
		}
		return nil, err
	}
	return value, nil
}

// marshalPretty is retained for non-body display helpers. It has the same
// bounded writer as captured JSON so form/query rendering cannot grow without
// limit either.
func marshalPretty(value any) ([]byte, error) {
	formatted, _, err := formatValueForDisplay(value, true, defaultJSONDisplayLimit)
	return formatted, err
}

func formatValueForDisplay(value any, showSensitive bool, limit int64) ([]byte, jsonDisplayMeta, error) {
	output, err := newLimitedDisplayBuffer(limit, 0)
	if err != nil {
		return nil, jsonDisplayMeta{}, err
	}
	formatter := valueDisplayFormatter{
		output:        output,
		showSensitive: showSensitive,
	}
	if err := formatter.writeValue(value, "", 0, false); err != nil {
		return nil, jsonDisplayMeta{}, err
	}
	return output.bytes(), formatter.meta, nil
}

type jsonDisplayFormatter struct {
	decoder       *json.Decoder
	output        *limitedDisplayBuffer
	showSensitive bool
	meta          jsonDisplayMeta
}

func (f *jsonDisplayFormatter) writeValue(key string, depth int, rootURL bool) error {
	if depth > maxJSONDisplayDepth {
		return errJSONDepth
	}
	token, err := f.decoder.Token()
	if err != nil {
		return err
	}
	if !f.showSensitive && key != "" && sensitiveKey(key) {
		if err := f.discardValue(token, depth); err != nil {
			return err
		}
		return f.writeJSONString(redactedValue)
	}
	if normalizeKey(key) == "er" && truthy(token) {
		f.meta.requestEncrypted = true
	}
	if rootURL {
		if text, ok := token.(string); ok {
			f.meta.rootURL = text
		}
	}

	switch typed := token.(type) {
	case json.Delim:
		switch typed {
		case '{':
			return f.writeObject(depth)
		case '[':
			return f.writeArray(depth)
		default:
			return errors.New("unexpected JSON delimiter")
		}
	case string:
		redacted, nestedMeta := redactJSONStringWithMeta(typed, f.showSensitive)
		f.meta.requestEncrypted = f.meta.requestEncrypted || nestedMeta.requestEncrypted
		return f.writeJSONString(redacted)
	case json.Number:
		return f.output.writeString(typed.String())
	case bool:
		if typed {
			return f.output.writeString("true")
		}
		return f.output.writeString("false")
	case nil:
		return f.output.writeString("null")
	default:
		return fmt.Errorf("unsupported JSON token %T", token)
	}
}

func (f *jsonDisplayFormatter) writeObject(depth int) error {
	if err := f.output.writeByte('{'); err != nil {
		return err
	}
	first := true
	for f.decoder.More() {
		keyToken, err := f.decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return errors.New("JSON object key is not a string")
		}
		if !first {
			if err := f.output.writeString(",\n"); err != nil {
				return err
			}
		} else {
			if err := f.output.writeByte('\n'); err != nil {
				return err
			}
		}
		if err := f.writeIndent(depth + 1); err != nil {
			return err
		}
		if err := f.writeJSONString(key); err != nil {
			return err
		}
		if err := f.output.writeString(": "); err != nil {
			return err
		}
		if err := f.writeValue(key, depth+1, depth == 0 && strings.EqualFold(key, "url")); err != nil {
			return err
		}
		first = false
	}
	end, err := f.decoder.Token()
	if err != nil {
		return err
	}
	if end != json.Delim('}') {
		return errors.New("JSON object is not terminated")
	}
	if !first {
		if err := f.output.writeByte('\n'); err != nil {
			return err
		}
		if err := f.writeIndent(depth); err != nil {
			return err
		}
	}
	return f.output.writeByte('}')
}

func (f *jsonDisplayFormatter) writeArray(depth int) error {
	if err := f.output.writeByte('['); err != nil {
		return err
	}
	first := true
	for f.decoder.More() {
		if !first {
			if err := f.output.writeString(",\n"); err != nil {
				return err
			}
		} else {
			if err := f.output.writeByte('\n'); err != nil {
				return err
			}
		}
		if err := f.writeIndent(depth + 1); err != nil {
			return err
		}
		if err := f.writeValue("", depth+1, false); err != nil {
			return err
		}
		first = false
	}
	end, err := f.decoder.Token()
	if err != nil {
		return err
	}
	if end != json.Delim(']') {
		return errors.New("JSON array is not terminated")
	}
	if !first {
		if err := f.output.writeByte('\n'); err != nil {
			return err
		}
		if err := f.writeIndent(depth); err != nil {
			return err
		}
	}
	return f.output.writeByte(']')
}

func (f *jsonDisplayFormatter) discardValue(token json.Token, depth int) error {
	if depth > maxJSONDisplayDepth {
		return errJSONDepth
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		for f.decoder.More() {
			if _, err := f.decoder.Token(); err != nil {
				return err
			}
			child, err := f.decoder.Token()
			if err != nil {
				return err
			}
			if err := f.discardValue(child, depth+1); err != nil {
				return err
			}
		}
		end, err := f.decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return errors.New("JSON object is not terminated")
		}
	case '[':
		for f.decoder.More() {
			child, err := f.decoder.Token()
			if err != nil {
				return err
			}
			if err := f.discardValue(child, depth+1); err != nil {
				return err
			}
		}
		end, err := f.decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return errors.New("JSON array is not terminated")
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	return nil
}

func (f *jsonDisplayFormatter) writeIndent(depth int) error {
	return f.output.writeString(strings.Repeat("  ", depth))
}

func (f *jsonDisplayFormatter) writeJSONString(value string) error {
	return writeJSONString(f.output, value)
}

type valueDisplayFormatter struct {
	output        *limitedDisplayBuffer
	showSensitive bool
	meta          jsonDisplayMeta
}

func (f *valueDisplayFormatter) writeValue(value any, key string, depth int, rootURL bool) error {
	if depth > maxJSONDisplayDepth {
		return errJSONDepth
	}
	if !f.showSensitive && key != "" && sensitiveKey(key) {
		return writeJSONString(f.output, redactedValue)
	}
	if normalizeKey(key) == "er" && truthy(value) {
		f.meta.requestEncrypted = true
	}
	if rootURL {
		if text, ok := value.(string); ok {
			f.meta.rootURL = text
		}
	}

	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for childKey := range typed {
			keys = append(keys, childKey)
		}
		sort.Strings(keys)
		if err := f.output.writeByte('{'); err != nil {
			return err
		}
		for i, childKey := range keys {
			if i == 0 {
				if err := f.output.writeByte('\n'); err != nil {
					return err
				}
			} else if err := f.output.writeString(",\n"); err != nil {
				return err
			}
			if err := f.output.writeString(strings.Repeat("  ", depth+1)); err != nil {
				return err
			}
			if err := writeJSONString(f.output, childKey); err != nil {
				return err
			}
			if err := f.output.writeString(": "); err != nil {
				return err
			}
			if err := f.writeValue(typed[childKey], childKey, depth+1, depth == 0 && strings.EqualFold(childKey, "url")); err != nil {
				return err
			}
		}
		if len(keys) > 0 {
			if err := f.output.writeByte('\n'); err != nil {
				return err
			}
			if err := f.output.writeString(strings.Repeat("  ", depth)); err != nil {
				return err
			}
		}
		return f.output.writeByte('}')
	case []any:
		if err := f.output.writeByte('['); err != nil {
			return err
		}
		for i, child := range typed {
			if i == 0 {
				if err := f.output.writeByte('\n'); err != nil {
					return err
				}
			} else if err := f.output.writeString(",\n"); err != nil {
				return err
			}
			if err := f.output.writeString(strings.Repeat("  ", depth+1)); err != nil {
				return err
			}
			if err := f.writeValue(child, "", depth+1, false); err != nil {
				return err
			}
		}
		if len(typed) > 0 {
			if err := f.output.writeByte('\n'); err != nil {
				return err
			}
			if err := f.output.writeString(strings.Repeat("  ", depth)); err != nil {
				return err
			}
		}
		return f.output.writeByte(']')
	case string:
		redacted, nestedMeta := redactJSONStringWithMeta(typed, f.showSensitive)
		f.meta.requestEncrypted = f.meta.requestEncrypted || nestedMeta.requestEncrypted
		return writeJSONString(f.output, redacted)
	case json.Number:
		return f.output.writeString(typed.String())
	case bool:
		if typed {
			return f.output.writeString("true")
		}
		return f.output.writeString("false")
	case nil:
		return f.output.writeString("null")
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return err
		}
		return f.output.writeString(string(encoded))
	default:
		return fmt.Errorf("unsupported display value %T", value)
	}
}

func writeJSONString(output *limitedDisplayBuffer, value string) error {
	if err := output.writeByte('"'); err != nil {
		return err
	}
	for _, runeValue := range value {
		switch runeValue {
		case '\\':
			if err := output.writeString(`\\`); err != nil {
				return err
			}
		case '"':
			if err := output.writeString(`\"`); err != nil {
				return err
			}
		case '\b':
			if err := output.writeString(`\b`); err != nil {
				return err
			}
		case '\f':
			if err := output.writeString(`\f`); err != nil {
				return err
			}
		case '\n':
			if err := output.writeString(`\n`); err != nil {
				return err
			}
		case '\r':
			if err := output.writeString(`\r`); err != nil {
				return err
			}
		case '\t':
			if err := output.writeString(`\t`); err != nil {
				return err
			}
		default:
			if runeValue < 0x20 {
				if err := output.writeString(fmt.Sprintf(`\u%04x`, runeValue)); err != nil {
					return err
				}
				continue
			}
			if err := output.writeString(string(runeValue)); err != nil {
				return err
			}
		}
	}
	return output.writeByte('"')
}

func redactValue(value any, key string, showSensitive bool) any {
	return redactValueAtDepth(value, key, showSensitive, 0)
}

func redactValueAtDepth(value any, key string, showSensitive bool, depth int) any {
	if !showSensitive && key != "" && sensitiveKey(key) {
		return redactedValue
	}
	if depth >= maxJSONDisplayDepth {
		return redactedValue
	}

	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for childKey, child := range typed {
			result[childKey] = redactValueAtDepth(child, childKey, showSensitive, depth+1)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i, child := range typed {
			result[i] = redactValueAtDepth(child, "", showSensitive, depth+1)
		}
		return result
	case string:
		return redactJSONString(typed, showSensitive)
	default:
		return value
	}
}

func redactJSONString(value string, showSensitive bool) string {
	redacted, _ := redactJSONStringWithMeta(value, showSensitive)
	return redacted
}

func redactJSONStringWithMeta(value string, showSensitive bool) (string, jsonDisplayMeta) {
	if showSensitive {
		trimmed := strings.TrimSpace(value)
		if len(trimmed) >= 1 && len(trimmed) <= maxNestedJSONDisplayBytes && (trimmed[0] == '{' || trimmed[0] == '[') {
			if _, meta, err := formatJSONLimited([]byte(trimmed), true, maxNestedJSONDisplayBytes); err == nil {
				return value, meta
			}
		}
		return value, jsonDisplayMeta{}
	}
	if !utf8.ValidString(value) {
		return unsafeTextPlaceholder, jsonDisplayMeta{}
	}
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= 1 && (trimmed[0] == '{' || trimmed[0] == '[') {
		// Nested JSON strings are common in NetEase request fields. Parse only a
		// small bounded copy; malformed, deep, or large nested values fail closed.
		if len(trimmed) > maxNestedJSONDisplayBytes {
			return unsafeTextPlaceholder, jsonDisplayMeta{}
		}
		formatted, meta, err := formatJSONLimited([]byte(trimmed), false, maxNestedJSONDisplayBytes)
		if err != nil {
			return unsafeTextPlaceholder, jsonDisplayMeta{}
		}
		start := strings.Index(value, trimmed)
		if start < 0 {
			return string(formatted), meta
		}
		return value[:start] + string(formatted) + value[start+len(trimmed):], meta
	}

	parsed, err := url.Parse(trimmed)
	if err == nil && (parsed.RawQuery != "" || parsed.User != nil) {
		redacted := redactURL(parsed, false)
		start := strings.Index(value, trimmed)
		if start < 0 {
			return redacted, jsonDisplayMeta{}
		}
		return value[:start] + redacted + value[start+len(trimmed):], jsonDisplayMeta{}
	}
	return string(redactText([]byte(value), false)), jsonDisplayMeta{}
}

func redactText(value []byte, showSensitive bool) []byte {
	if showSensitive || len(value) == 0 {
		return append([]byte(nil), value...)
	}
	if !utf8.Valid(value) {
		return []byte(unsafeTextPlaceholder)
	}
	redacted := sensitiveLine.ReplaceAll(value, []byte(`${1}`+redactedValue))
	redacted = sensitiveInline.ReplaceAll(redacted, []byte(`${1}${2}`+redactedValue))
	return redacted
}

func redactDiagnostic(value []byte, showSensitive bool) []byte {
	if showSensitive || len(value) == 0 {
		return append([]byte(nil), value...)
	}
	if !utf8.Valid(value) {
		return []byte(unsafeTextPlaceholder)
	}
	redacted := diagnosticURL.ReplaceAllFunc(value, func(match []byte) []byte {
		parsed, err := url.Parse(string(match))
		if err != nil {
			return []byte(unsafeTextPlaceholder)
		}
		return []byte(redactURL(parsed, false))
	})
	return redactText(redacted, false)
}

func safeBodyPlaceholder(limit int64, reason string) []byte {
	if limit <= 0 {
		return []byte{}
	}
	value := []byte("<" + reason + ">")
	if int64(len(value)) <= limit {
		return value
	}
	return append([]byte(nil), value[:limit]...)
}

// escapeLogField keeps values written in one-line metadata fields from
// introducing fake records or terminal control sequences.
func escapeLogField(value string) string {
	var buffer strings.Builder
	buffer.Grow(len(value))
	for len(value) > 0 {
		runeValue, size := utf8.DecodeRuneInString(value)
		if runeValue == utf8.RuneError && size == 1 {
			fmt.Fprintf(&buffer, `\x%02x`, value[0])
			value = value[1:]
			continue
		}
		value = value[size:]
		switch runeValue {
		case '\n':
			buffer.WriteString(`\n`)
		case '\r':
			buffer.WriteString(`\r`)
		case '\t':
			buffer.WriteString(`\t`)
		case 0x1b:
			buffer.WriteString(`\x1b`)
		default:
			if unicode.IsControl(runeValue) {
				fmt.Fprintf(&buffer, `\u%04x`, runeValue)
			} else {
				buffer.WriteRune(runeValue)
			}
		}
	}
	return buffer.String()
}

func valuesToValue(values url.Values) map[string]any {
	result := make(map[string]any, len(values))
	for key, entries := range values {
		if len(entries) == 1 {
			result[key] = entries[0]
			continue
		}
		items := make([]any, len(entries))
		for i, entry := range entries {
			items[i] = entry
		}
		result[key] = items
	}
	return result
}

func sensitiveKey(key string) bool {
	normalized := normalizeKey(key)
	if normalized == "" {
		return false
	}
	switch normalized {
	case "authorization", "proxyauthorization", "cookie", "setcookie",
		"password", "passwd", "pwd", "credential", "credentials", "secret",
		"musicu", "musica", "musicru", "musicra", "musicat",
		"phone", "phonenumber", "cellphone", "mobile",
		"email", "imei", "imsi", "deviceid", "deviceidentifier", "captcha",
		"verificationcode", "smscode":
		return true
	}
	return strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "passwd") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "csrf") ||
		strings.Contains(normalized, "signature") ||
		strings.Contains(normalized, "accesskey") ||
		strings.Contains(normalized, "apikey") ||
		strings.Contains(normalized, "sessionkey") ||
		strings.Contains(normalized, "sskey") ||
		strings.HasSuffix(normalized, "cookie")
}

func normalizeKey(key string) string {
	var builder strings.Builder
	builder.Grow(len(key))
	for _, r := range strings.ToLower(key) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func valuesRequestEncrypted(values url.Values) bool {
	for key, entries := range values {
		if normalizeKey(key) != "er" {
			continue
		}
		for _, entry := range entries {
			if truthy(entry) {
				return true
			}
		}
	}
	return false
}

func valueRequestEncrypted(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if normalizeKey(key) == "er" && truthy(child) {
				return true
			}
			if valueRequestEncrypted(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if valueRequestEncrypted(child) {
				return true
			}
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if len(trimmed) >= 2 && (trimmed[0] == '{' || trimmed[0] == '[') {
			if formatted, meta, err := formatJSONLimited([]byte(trimmed), false, defaultJSONDisplayLimit); err == nil {
				_ = formatted
				return meta.requestEncrypted
			}
		}
	}
	return false
}

func truthy(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true
		}
	case json.Number:
		floatValue, err := typed.Float64()
		return err == nil && floatValue != 0
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	}
	return false
}
