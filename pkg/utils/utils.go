// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package utils

import (
	"bytes"
	"compress/gzip"
	"crypto/md5" //nolint:gosec // NetEase exposes MD5 checksums for media integrity verification.
	cryptorand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/google/uuid"
)

const (
	B int64 = 1 << (10 * iota)
	KB
	MB
	GB
	TB
	PB
)

const (
	defMaxGzipSize    = MB * 32
	jsessionIDCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/\\"
)

var (
	homeDir          string
	parseBytesRegexp = regexp.MustCompile(`(?i)^(\d+)([a-zA-Z]*)$`)
	filenameRegexp   = regexp.MustCompile("[\\\\/:*?\"<>|]")
	unitMap          = map[string]int64{
		"B":  B,
		"K":  KB,
		"KB": KB,
		"M":  MB,
		"MB": MB,
	}
	musicExts = map[string]struct{}{
		".mp3":  {},
		".flac": {},
		".wav":  {},
		".m4a":  {},
		".ogg":  {},
		".ape":  {},
		".wma":  {},
		".aac":  {},
		".aiff": {},
		".ac3":  {},
		".dts":  {},
		".wv":   {},
		".mpc":  {},
		".opus": {},
		".mka":  {},
		".m3u":  {},
		".m3u8": {},
		".pls":  {},
	}
)

func init() {
	dir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	homeDir = dir
}

// ParseBytes 将输入字符串转换为字节数.
func ParseBytes(input string) (int64, error) {
	if input == "" {
		return 0, nil
	}

	matches := parseBytesRegexp.FindStringSubmatch(input)
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid input format: %s", input)
	}

	valueStr := matches[1]
	unit := matches[2]

	// 转换数字部分
	value, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %s", valueStr)
	}

	// 默认单位是字节
	if unit == "" {
		unit = "B"
	}

	// 将单位转换为小写
	unit = strings.ToUpper(unit)

	// 获取对应的字节数乘数
	multiplier, exists := unitMap[unit]
	if !exists {
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}

	if value > math.MaxInt64/multiplier {
		return 0, fmt.Errorf("byte value overflows int64: %s", input)
	}
	return value * multiplier, nil
}

// FileExists 判断文件是否存在.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && !info.IsDir()
}

// DirExists 判断目录是否存在.
func DirExists(dirPath string) bool {
	info, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && info.IsDir()
}

// IsFile 判断是否为文件.
func IsFile(path string) bool {
	d, err := os.Stat(path)
	if err != nil {
		return false
	}

	if d.IsDir() {
		return false
	}
	return true
}

func MkdirIfNotExist(path string, perm os.FileMode) error {
	if !DirExists(path) {
		return os.MkdirAll(path, perm)
	}
	return nil
}

// ExpandTilde 扩展波浪号路径.
func ExpandTilde(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("UserHomeDir: %w", err)
	}

	// 处理纯 ~ 或 ~/ 路径
	if path == "~" || (len(path) >= 2 && (path[1] == '/' || path[1] == '\\')) {
		return filepath.Join(home, path[1:]), nil
	}

	// todo:
	// ~file 分为两种情况:
	// 1：如果 file 是用户名，则指向该用户的主目录。
	// 例如：~bob 表示用户 bob 的主目录（如 /home/bob）。
	// 2：如果 file 不是用户名，则可能被解析为字面量（即一个实际的文件名）。
	// 例如：~file.txt 可能被误认为是用户 file.txt 的主目录，但更常见的是被当作普通文件名处理（需转义或引号包裹）。
	return path, nil
}

// CheckPath 检查路径是否存在，并返回是否为目录, 支持~路径检测.
func CheckPath(path string) (bool, bool, error) {
	expandedPath, err := ExpandTilde(path)
	if err != nil {
		return false, false, fmt.Errorf("ExpandTilde: %w", err)
	}

	stat, err := os.Stat(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil // Path does not exist
		}
		return false, false, err // Some other error occurred
	}
	// Path exists, determine if it is a directory
	return true, stat.IsDir(), nil
}

func MD5Hex(data []byte) (string, error) {
	digest := md5.Sum(data) //nolint:gosec // This is a compatibility checksum, not a security primitive.
	return hex.EncodeToString(digest[:]), nil
}

// Ternary is a generic function that mimics a ternary expression.
func Ternary[T any](condition bool, trueVal, falseVal T) T {
	if condition {
		return trueVal
	}
	return falseVal
}

func IsUnique[T comparable](arr []T) bool {
	set := make(map[T]struct{})
	for _, v := range arr {
		if _, ok := set[v]; ok {
			return false
		}

		set[v] = struct{}{}
	}
	return true
}

func IsMusicExt(ext string) bool {
	_, exist := musicExts[filepath.Ext(ext)]
	return exist
}

func DetectContentType(data []byte, ext string) string {
	if ext == ".flac" {
		return "audio/flac"
	}

	var (
		ct = http.DetectContentType(data)
		k  = strings.SplitN(ct, "/", 1)
	)
	if len(k) > 0 && k[0] != "audio" {
		ct = "audio/mpeg"
	}
	return ct
}

func SplitSlice[T any](input []T, chunkSize int) ([][]T, error) {
	if chunkSize <= 0 {
		return nil, errors.New("chunkSize must be greater than 0")
	}

	var result [][]T

	for i := 0; i < len(input); i += chunkSize {
		end := min(i+chunkSize, len(input))

		result = append(result, input[i:end])
	}
	return result, nil
}

// TimeUntilMidnight 计算当前时间到明天零点的时间差.
func TimeUntilMidnight(timeZone string) (time.Duration, error) {
	var (
		loc *time.Location
		err error
	)

	// Get the time zone location
	if timeZone == "" {
		loc = time.Local
	} else {
		loc, err = time.LoadLocation(timeZone)
		if err != nil {
			return 0, fmt.Errorf("invalid time zone: %w", err)
		}
	}

	now := time.Now().In(loc)
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, loc)
	return midnight.Sub(now), nil
}

// Filename 清理文件名中的非法字符.
func Filename(path string, replacement ...string) string {
	path = strings.TrimSpace(path)
	if len(replacement) > 0 {
		return filenameRegexp.ReplaceAllString(path, replacement[0])
	}
	return filenameRegexp.ReplaceAllString(path, "")
}

// IsGzipHeader 判断数据是否为 Gzip 格式。
// 参数:
//
//	data: 待检查的字节切片
//	strict: 可选参数，若传入且为 true，则额外检查压缩方法是否为 DEFLATE (0x08)
//
// 返回: 如果符合 Gzip 格式则返回 true，否则返回 false.
func IsGzipHeader(data []byte, strict ...bool) bool {
	if len(data) < 2 {
		return false
	}

	if data[0] != 0x1F || data[1] != 0x8B {
		return false
	}

	// 默认仅检查 gzip 魔数；严格模式额外验证压缩方法。
	if len(strict) == 0 || !strict[0] {
		return true
	}

	return len(data) >= 3 && data[2] == 0x08
}

// GzipReader 如果不是gzip则直接返回。
func GzipReader(plaintext []byte, maxSize ...int64) ([]byte, error) {
	if !IsGzipHeader(plaintext) {
		return plaintext, nil
	}

	reader, err := gzip.NewReader(bytes.NewReader(plaintext))
	if err != nil {
		return nil, fmt.Errorf("gzip.NewReader: %w", err)
	}

	// 防止压缩炸弹攻击
	limit := defMaxGzipSize
	if len(maxSize) > 0 && maxSize[0] > 0 {
		limit = maxSize[0]
	}

	data, readErr := io.ReadAll(io.LimitReader(reader, limit+1))

	closeErr := reader.Close()
	if readErr != nil {
		return nil, fmt.Errorf("gzip.ReadAll: %w", errors.Join(readErr, closeErr))
	}

	if closeErr != nil {
		return nil, fmt.Errorf("gzip.Close: %w", closeErr)
	}

	return data, nil
}

// GenerateWNMCID 生成WNMCID(貌似是Netease Machine Client ID缩写)
// 生成规则: 6位随机小写字母 + 当前时间戳（13位毫秒） + 默认抓取版本号 + n(可能是客户端类型，或风控状态标识目前已知有:0、4值)
// 例如: "abcdef.1633557080686.01.0"
// 作用: 貌似是网易云音乐的抓取标识,或者用于爬虫标识等作用.
func GenerateWNMCID() string {
	const (
		crawlerVersion = "01"
		alphabet       = "abcdefghijklmnopqrstuvwxyz"
	)

	prefix := make([]byte, 6)
	for i := range prefix {
		prefix[i] = alphabet[rand.IntN(len(alphabet))]
	}

	return fmt.Sprintf("%s.%d.%s.0", prefix, time.Now().UnixMilli(), crawlerVersion)
}

// GenerateChainId 生成ChainId 用于web login
// 规则: "{version}_{s_device_id}_{platform}_{action}_{timestamp}" .
func GenerateChainId(deviceId string) string {
	return fmt.Sprintf("v1_%s_web_%s_%d", deviceId, "login", time.Now().UnixMilli())
}

// GenerateDeviceId 生成设备ID
// 目前发现以下几种种格式设备id
//
// - 格式1: 52位十六进制字符串 (大写)
// 例如: D82036205567D95288D26BCF3797FE22D8A7634320474768C7ED
// 场景:
//
// - 格式2: UUIDv5和UUIDv4拼接而成, 中间用|分隔, UUID格式为8-4-4-4-12的十六进制字符串 (大写)
// 场景: 在weapi pc端场景碰到。
// 例如: 7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62|5FD718A3-0602-4389-B612-EBEFAA7F108B.
//
// - 格式3: CTAyOjAwOjAwOjAwOjAwOjAwCTBiYmM3OTJiNDk4Y2FjYTcJNWU5YWViZGNmODdiNWQzMg== (解析 URL Encode后的值)
// 场景: eapi手机端发现该样式值。
// 例如: (以下结论基于ai未必准确)大概如下需要确认 Base64(IMEI\tMAC\Android_ID\tDeviceFingerprintHash)
// 红米android12: CTAyOjAwOjAwOjAwOjAwOjAwCTBiYmM3OTJiNDk4Y2FjYTcJNWU5YWViZGNmODdiNWQzMg==-> Base64(02:00:00:00:00:00\t0bbc792b498caca7\t5e9aebdcf87b5d32) .
// 三星设备android9: MzUxNTY0MTAxMTE4NDEyCTA4OjAwOjI3OjRmOjEyOmJhCThjMzczZDE5ODk3ODc2M2EJOTZiOGEzZjBmYzgyMDgxNw== -> Base64(351564101118412\t08:00:27:4f:12:ba\t8c373d198978763a\t96b8a3f0fc820817)
// 说明：
// 1. IMEI可以为空，如果没有则不参与计算。安卓10+对IMEI、MAC等硬件访问权限大幅收紧，逐渐演变为更多依赖 Android_ID、OAID、应用安装 ID 以及设备指纹哈希，而不再直接包含可读取的 IMEI 或真实 MAC。
// 2. MAC地址Android从6.0开始逐步限制，部分厂商可绕过，到10彻底限制，应用无法读取真实 MAC, 其中"02:00:00:00:00:00"这是 Android 默认 MAC 占位值。另外mac地址“08:00:27:xx:xxx:xx”开头基本基本属于Oracle VirtualBox网卡MAC.
// 3. 安卓id可信度较高但不一定正确。
// 4. DeviceFingerprintHash,计算方式不明确通常是根据设备相关信息组合而出HASH(IMEI,MAC,Android,OAID,Brand,Model,Device,...).
// 结论: 可以看到这种格式设备id包含手机一些相关信息，如果接口请求参数和设备id中的信息对应不上风控风险会上升。
//
// - 格式4: YD-g6nR2rDf+flFV1UEBVLCNo6WtZ22c3gy
// 场景: mac pc弹出网页在浏览器中查看的值。
//
// - 格式5: 4cdb39bf34a848781b89663e1e546b8b
// 场景: mac pc端中eapi中,另外再header中发现了格式2中得设备id 7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62|5FD718A3-0602-4389-B612-EBEFAA7F108B 同一个请求中发现了2种设备id。
func GenerateDeviceId(isLong ...bool) string {
	if len(isLong) > 0 && isLong[0] {
		uuidV4 := uuid.New()
		uuidV5 := uuid.NewSHA1(uuid.NameSpaceURL, []byte(uuidV4.URN()))
		return strings.ToUpper(uuidV5.String() + "|" + uuidV4.String())
	}

	const hexChars = "0123456789ABCDEF"

	b := make([]byte, 52)
	for i := range b {
		b[i] = hexChars[rand.IntN(len(hexChars))]
	}
	return string(b)
}

// GenerateRequestId 请求id.
func GenerateRequestId() string {
	return fmt.Sprintf("%d_%04d", time.Now().UnixNano()/1000000, rand.IntN(1000))
}

func BaseDir(elem ...string) string {
	elems := append([]string{homeDir, ".ncmctl"}, elem...)
	return filepath.Join(elems...)
}

type NVIDInput struct {
	DateNowMillis int64   // Date.getTime()
	MathRandom    float64 // Math.random()
	Location      string  // document.location
	Referrer      string  // document.referrer
	ScreenWidth   int     // screen.width
	ScreenHeight  int     // screen.height
	UserAgent     string  // navigator.userAgent
	Cookie        string  // document.cookie
	ClientWidth   int     // document.body.clientWidth
	ClientHeight  int     // document.body.clientHeight
}

// strToEnt 与网页 JS 中的 str_to_ent() 保持一致。
// ASCII(<=255) 字符保持不变；
// >255 的 Unicode 字符转换成 HTML 十进制实体，例如：
// "你" -> "&#20320;".
func strToEnt(s string) string {
	var b strings.Builder

	for _, v := range utf16.Encode([]rune(s)) {
		if v > 255 {
			b.WriteString("&#")
			b.WriteString(strconv.FormatUint(uint64(v), 10))
			b.WriteByte(';')
		} else {
			b.WriteByte(byte(v))
		}
	}

	return b.String()
}

// GenerateNVID 根据网易网页算法生成 _ntes_nuid 访客哈希。
// _ntes_nnid 在该哈希后追加一个13位毫秒时间戳。
// ref: https://github.com/Binaryify/NeteaseCloudMusicApi/pull/291 .
//
//nolint:gocritic // Value input avoids a nil boundary; visitor ID generation is not a hot path.
func GenerateNVID(in NVIDInput) string {
	var b strings.Builder

	// JavaScript evaluates Date.getTime() + Math.random() numerically before string concatenation.
	b.WriteString(strconv.FormatFloat(float64(in.DateNowMillis)+in.MathRandom, 'f', -1, 64))

	// document.location
	b.WriteString(in.Location)

	// document.referrer
	b.WriteString(in.Referrer)

	// screen.width
	b.WriteString(strconv.Itoa(in.ScreenWidth))

	// screen.height
	b.WriteString(strconv.Itoa(in.ScreenHeight))

	// navigator.userAgent
	b.WriteString(in.UserAgent)

	// document.cookie
	b.WriteString(in.Cookie)

	// document.body.clientWidth
	b.WriteString(strconv.Itoa(in.ClientWidth))

	b.WriteByte(':')

	// document.body.clientHeight
	b.WriteString(strconv.Itoa(in.ClientHeight))

	sum := md5.Sum([]byte(strToEnt(b.String()))) //nolint:gosec // NetEase's visitor ID protocol requires MD5.
	return hex.EncodeToString(sum[:])
}

// GenerateFakeNVID 生成假 _ntes_nnid，_ntes_nuid值,在weapi中cookie使用.
// 由于id最终结果是hash值，hash具有不可逆性因此直接使用随机生成即可。
// 要保证真时则可使用 GenerateNVID() 方法。
// 示例:
// _ntes_nnid: 47eabde2b621ff2848404b66e558ca99,1773424950453
// _ntes_nuid: 47eabde2b621ff2848404b66e558ca99 .
func GenerateFakeNVID() (string, string, error) {
	var buf [16]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return "", "", fmt.Errorf("read random bytes: %w", err)
	}

	nuid := hex.EncodeToString(buf[:])
	nnid := fmt.Sprintf("%s,%v", nuid, time.Now().UnixMilli())
	return nnid, nuid, nil
}

func randomJsessionStr(length int) (string, error) {
	var b strings.Builder
	b.Grow(length)

	random := make([]byte, length)
	if _, err := cryptorand.Read(random); err != nil {
		return "", err
	}

	for _, v := range random {
		b.WriteByte(jsessionIDCharset[int(v)%len(jsessionIDCharset)])
	}

	return b.String(), nil
}

// GenerateFakeJSessionIDWYYY 生成一个格式与网易 JSESSIONID-WYYY 相似的值,在weapi中cookie中使用。
// 注意: 这不是一个真实有效的算法。
// ref: https://github.com/Binaryify/NeteaseCloudMusicApi/pull/291
// 格式: <RandomToken>:<UnixMilli>
// 示例: 3slBRc1XFjYNOvMxBxdRGqy8Z4DX4G4jgyDvGm1ASpoY4BuxNzk5oQv81JosSsqnXuYjeXbpwVl%5Co%2Bido9...:1785223028570 .
func GenerateFakeJSessionIDWYYY() (string, error) {
	// 长度约大约 178~182，这里取 180。
	const tokenLength = 180

	token, err := randomJsessionStr(tokenLength)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%d", token, time.Now().UnixMilli()), nil
}
