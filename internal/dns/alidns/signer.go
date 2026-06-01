package alidns

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

const (
	formatJSON       = "JSON"
	apiVersion       = "2015-01-09"
	signatureMethod  = "HMAC-SHA1"
	signatureVersion = "1.0"
)

// Sign 生成阿里云 API 请求签名（HMAC-SHA1 + Base64）
// 参考: https://help.aliyun.com/document_detail/29747.html
func Sign(params map[string]string, accessKeySecret string) string {
	// 1. 排序参数
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "Signature" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 2. 拼接规范化查询字符串
	var canonicalizedParts []string
	for _, k := range keys {
		canonicalizedParts = append(canonicalizedParts,
			fmt.Sprintf("%s=%s", percentEncode(k), percentEncode(params[k])))
	}
	canonicalizedQueryString := strings.Join(canonicalizedParts, "&")

	// 3. 构造待签名字符串
	stringToSign := fmt.Sprintf("GET&%s&%s",
		percentEncode("/"),
		percentEncode(canonicalizedQueryString))

	// 4. HMAC-SHA1 签名
	key := []byte(accessKeySecret + "&")
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return signature
}

// percentEncode 阿里云特殊 URL 编码规则
func percentEncode(s string) string {
	var result strings.Builder
	for _, b := range []byte(s) {
		if isUnreserved(b) {
			result.WriteByte(b)
		} else {
			result.WriteString(fmt.Sprintf("%%%02X", b))
		}
	}
	return result.String()
}

func isUnreserved(b byte) bool {
	return (b >= 'A' && b <= 'Z') ||
		(b >= 'a' && b <= 'z') ||
		(b >= '0' && b <= '9') ||
		b == '-' || b == '.' || b == '_' || b == '~'
}

func generateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// buildParams 组装通用请求参数
func buildParams(accessKeyID, action string) map[string]string {
	return map[string]string{
		"Format":           formatJSON,
		"Version":          apiVersion,
		"AccessKeyId":      accessKeyID,
		"SignatureMethod":  signatureMethod,
		"SignatureVersion": signatureVersion,
		"SignatureNonce":   generateNonce(),
		"Action":           action,
	}
}

// MergeParams 合并通用参数和业务参数
func MergeParams(base, extra map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range extra {
		merged[k] = v
	}
	return merged
}

// BuildSignedURL 构建签名后的完整请求 URL
func BuildSignedURL(endpoint string, params map[string]string, secret string) string {
	params["Signature"] = Sign(params, secret)

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", percentEncode(k), percentEncode(params[k])))
	}

	return fmt.Sprintf("https://%s/?%s", endpoint, strings.Join(parts, "&"))
}
