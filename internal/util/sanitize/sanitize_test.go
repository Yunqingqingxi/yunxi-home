package sanitize

import (
	"testing"
)

func TestDomain(t *testing.T) {
	tests := []struct {
		input    string
		wantOK   bool
		wantVal  string
	}{
		{"example.com", true, "example.com"},
		{"sub.example.com", true, "sub.example.com"},
		{"*.example.com", true, "*.example.com"},
		{"", false, ""},
		{"   ", false, ""},
		{"not a domain", false, ""},
		{"https://evil.com", false, ""},
		{"a.co", true, "a.co"}, // valid short domain
	}
	for _, tt := range tests {
		got, err := Domain(tt.input)
		if tt.wantOK && err != nil {
			t.Errorf("Domain(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.wantOK && err == nil {
			t.Errorf("Domain(%q) expected error, got nil", tt.input)
		}
		if tt.wantOK && got != tt.wantVal {
			t.Errorf("Domain(%q) = %q, want %q", tt.input, got, tt.wantVal)
		}
	}
}

func TestRecordType(t *testing.T) {
	tests := []struct {
		input  string
		wantOK bool
	}{
		{"A", true}, {"AAAA", true}, {"CNAME", true}, {"MX", true},
		{"TXT", true}, {"NS", true}, {"SRV", true}, {"CAA", true},
		{"UNKNOWN", false}, {"", false}, {"a", true}, // case insensitive
	}
	for _, tt := range tests {
		_, err := RecordType(tt.input)
		if tt.wantOK && err != nil {
			t.Errorf("RecordType(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.wantOK && err == nil {
			t.Errorf("RecordType(%q) expected error", tt.input)
		}
	}
}

func TestString(t *testing.T) {
	_, err := String("hello", 100)
	if err != nil {
		t.Errorf("String valid unexpected error: %v", err)
	}
	_, err = String("", 100)
	if err == nil {
		t.Error("String empty should error")
	}
	_, err = String("toolong", 3)
	if err == nil {
		t.Error("String too long should error")
	}
}

func TestPathSanitize(t *testing.T) {
	tests := []struct {
		input  string
		wantOK bool
	}{
		{"/home/user", true},
		{"/", true},
		{"", true}, // becomes "/"
		{"../etc/passwd", false},
		{"/path/with/null\x00", false},
	}
	for _, tt := range tests {
		_, err := PathSanitize(tt.input)
		if tt.wantOK && err != nil {
			t.Errorf("PathSanitize(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.wantOK && err == nil {
			t.Errorf("PathSanitize(%q) expected error", tt.input)
		}
	}
}

func TestEmail(t *testing.T) {
	tests := []struct {
		input  string
		wantOK bool
	}{
		{"user@example.com", true},
		{"", false},
		{"notanemail", false},
		{"@missinguser.com", false},
	}
	for _, tt := range tests {
		_, err := Email(tt.input)
		if tt.wantOK && err != nil {
			t.Errorf("Email(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.wantOK && err == nil {
			t.Errorf("Email(%q) expected error", tt.input)
		}
	}
}

func TestPort(t *testing.T) {
	if err := Port(80); err != nil {
		t.Errorf("Port(80) unexpected error: %v", err)
	}
	if err := Port(0); err == nil {
		t.Error("Port(0) should error")
	}
	if err := Port(99999); err == nil {
		t.Error("Port(99999) should error")
	}
}

func TestClamp(t *testing.T) {
	if got := Clamp(5, 1, 10); got != 5 {
		t.Errorf("Clamp(5,1,10) = %d", got)
	}
	if got := Clamp(0, 1, 10); got != 1 {
		t.Errorf("Clamp(0,1,10) = %d", got)
	}
	if got := Clamp(100, 1, 10); got != 10 {
		t.Errorf("Clamp(100,1,10) = %d", got)
	}
}

func TestDedupe(t *testing.T) {
	got := Dedupe([]string{"a", "b", "a", "c", "b"})
	if len(got) != 3 {
		t.Errorf("Dedupe len = %d, want 3", len(got))
	}
}

func TestURL(t *testing.T) {
	tests := []struct {
		input  string
		wantOK bool
	}{
		{"https://example.com", true},
		{"http://localhost:8080", true},
		{"ftp://bad.com", false},
		{"", false},
		{"not-a-url", false},
	}
	for _, tt := range tests {
		_, err := URL(tt.input)
		if tt.wantOK && err != nil {
			t.Errorf("URL(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.wantOK && err == nil {
			t.Errorf("URL(%q) expected error", tt.input)
		}
	}
}
