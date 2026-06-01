package ipdetect

import (
	"testing"
)

func TestIsValidIPv6(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"2409:8a38:8434:cf24:c0ff:d265:3f51:822c", true},
		{"::1", true},
		{"fe80::1", true},
		{"2001:db8::1", true},
		{"192.168.1.1", false},
		{"not an ip", false},
		{"", false},
		{"  2409:8a38:8434:cf24:c0ff:d265:3f51:822c  ", true},
		{"::ffff:192.0.2.1", true}, // IPv4-mapped IPv6
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsValidIPv6(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidIPv6(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsValidIPv4(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"256.1.1.1", false},
		{"not an ip", false},
		{"", false},
		{"2409:8a38:8434:cf24:c0ff:d265:3f51:822c", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsValidIPv4(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidIPv4(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsIPChanged(t *testing.T) {
	if IsIPChanged("1.1.1.1", "1.1.1.1") {
		t.Error("same IP should not be detected as changed")
	}
	if !IsIPChanged("1.1.1.1", "2.2.2.2") {
		t.Error("different IP should be detected as changed")
	}
	if IsIPChanged("", "") {
		t.Error("both empty should not be detected as changed")
	}
}

func TestDetectorNewDetector(t *testing.T) {
	d := NewDetector(nil)
	if d == nil {
		t.Fatal("NewDetector returned nil")
	}
	// Verify basic functionality: SetCachedIP and GetCachedIP
	d.SetCachedIP("example.com", "2409::1")
	ip, ok := d.GetCachedIP("example.com")
	if !ok || ip != "2409::1" {
		t.Errorf("GetCachedIP failed: %q, %v", ip, ok)
	}
}

func TestDetectorGetCachedIP(t *testing.T) {
	d := NewDetector(nil)
	d.SetCachedIP("example.com", "2409::1")

	ip, ok := d.GetCachedIP("example.com")
	if !ok || ip != "2409::1" {
		t.Errorf("GetCachedIP failed: %q, %v", ip, ok)
	}

	// 不存在的 key
	_, ok = d.GetCachedIP("nonexistent")
	if ok {
		t.Error("should not find nonexistent key")
	}
}
