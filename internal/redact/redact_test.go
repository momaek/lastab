package redact

import (
	"strings"
	"testing"
)

func TestMasks(t *testing.T) {
	cases := []struct{ in, want string }{
		{"-H 'Authorization: Bearer abcdef0123456789'", "-H 'Authorization: Bearer " + Mask + "'"},
		{"Authorization: Basic dXNlcjpwYXNz", "Authorization: Basic " + Mask},
		{`{"api_key": "abcd1234efgh"}`, `{"api_key": "` + Mask + `"}`},
		{"OPENAI_API_KEY=sk-proj-abcdefghijklmnop1234", "OPENAI_API_KEY=" + Mask},
		{"password: hunter22", "password: " + Mask},
		{"key ghp_abcdefghijklmnopqrstuvwxyz0123", "key " + Mask},
		{"AKIAABCDEFGHIJKLMNOP", Mask},
		{"x sk-ant-api03-abcdefghijklmnopqrst y", "x " + Mask + " y"},
		{"jwt eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N", "jwt " + Mask},
	}
	for _, c := range cases {
		got, ok := Text(c.in)
		if !ok || got != c.want {
			t.Errorf("Text(%q) = %q, %v; want %q", c.in, got, ok, c.want)
		}
	}
}

func TestPrivateKey(t *testing.T) {
	in := "-----BEGIN RSA PRIVATE KEY-----\nMIIEow\nabc\n-----END RSA PRIVATE KEY-----"
	got := String(in)
	if strings.Contains(got, "MIIEow") || !strings.HasPrefix(got, "-----BEGIN RSA PRIVATE KEY-----") {
		t.Errorf("got %q", got)
	}
}

// Ordinary text must be left alone.
func TestNoFalsePositives(t *testing.T) {
	for _, in := range []string{
		"周会纪要：发版前检查",
		"the token bucket algorithm",
		"curl -X POST https://api.example.com/v2/orders/search",
		`{"order_id": "A-20391", "status": "paid"}`,
		"password reset flow needs review",
		"task-list: secret santa",
		"sk-short",
	} {
		if got, ok := Text(in); ok {
			t.Errorf("Text(%q) masked: %q", in, got)
		}
	}
}
