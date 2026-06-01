package alidns

import (
	"testing"
)

func TestPercentEncode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"abc", "abc"},
		{"abc123", "abc123"},
		{"hello world", "hello%20world"},
		{"test@example.com", "test%40example.com"},
		{"/", "%2F"},
		{"&", "%26"},
		{"=", "%3D"},
		{"+", "%2B"},
		{"*", "%2A"},
		{"~", "~"},
		{"-_./", "-_.%2F"},
		{"中文", "%E4%B8%AD%E6%96%87"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := percentEncode(tt.input)
			if result != tt.expected {
				t.Errorf("percentEncode(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsUnreserved(t *testing.T) {
	if !isUnreserved('a') {
		t.Error("'a' should be unreserved")
	}
	if !isUnreserved('Z') {
		t.Error("'Z' should be unreserved")
	}
	if !isUnreserved('0') {
		t.Error("'0' should be unreserved")
	}
	if isUnreserved(' ') {
		t.Error("space should be reserved")
	}
	if isUnreserved('@') {
		t.Error("'@' should be reserved")
	}
}

func TestSign(t *testing.T) {
	params := map[string]string{
		"Format":           "JSON",
		"Version":          "2015-01-09",
		"AccessKeyId":      "testid",
		"SignatureMethod":  "HMAC-SHA1",
		"Timestamp":        "2016-02-23T12:46:24Z",
		"SignatureVersion": "1.0",
		"SignatureNonce":   "3ee8c1b8-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
		"Action":           "DescribeDomainRecords",
	}

	sig := Sign(params, "testsecret")
	if sig == "" {
		t.Error("signature should not be empty")
	}
	if sig == "testsecret" {
		t.Error("signature should not be the secret itself")
	}
}

func TestBuildParams(t *testing.T) {
	params := buildParams("test-key", "DescribeDomainRecords")

	requiredKeys := []string{"Format", "Version", "AccessKeyId", "SignatureMethod",
		"SignatureVersion", "SignatureNonce", "Action"}

	for _, key := range requiredKeys {
		if _, ok := params[key]; !ok {
			t.Errorf("missing required parameter: %s", key)
		}
	}

	if params["Action"] != "DescribeDomainRecords" {
		t.Errorf("expected Action=DescribeDomainRecords, got %s", params["Action"])
	}
	if params["Format"] != "JSON" {
		t.Errorf("expected Format=JSON, got %s", params["Format"])
	}
}

func TestGenerateNonce(t *testing.T) {
	n1 := generateNonce()
	n2 := generateNonce()

	if n1 == "" || n2 == "" {
		t.Error("nonce should not be empty")
	}
	if n1 == n2 {
		t.Error("two nonces should be different")
	}
	if len(n1) != 32 {
		t.Errorf("nonce length should be 32, got %d", len(n1))
	}
}

func TestAPIError(t *testing.T) {
	err := &APIError{Code: "TestCode", Message: "测试错误", RequestID: "12345"}
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
	if !IsAPIError(err, "TestCode") {
		t.Error("IsAPIError should return true for matching code")
	}
	if IsAPIError(err, "OtherCode") {
		t.Error("IsAPIError should return false for non-matching code")
	}
}
