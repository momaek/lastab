package redact

import (
	"math/rand/v2"
	"testing"
)

func TestBareTokens(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"Gpws2uEk7F6gIMi7NlweDHt", Mask},
		{"key: Gpws2uEk7F6gIMi7NlweDHt\n", "key: " + Mask + "\n"},
		{"hazmmazm23@gmail.com----qmPh3kZ8vTn2Lx9Rw4Ya", "hazmmazm23@gmail.com----" + Mask},
		{"token is NhBdTfK3pL9mQ2rV8sX1, keep it", "token is " + Mask + ", keep it"},
		{"aGVsbG8gd29ybGQ1MjM0NTY3OA==", Mask},
		{"https://x.dev/cb?code=Gpws2uEk7F6gIMi7NlweDHt&x=1", "https://x.dev/cb?code=" + Mask + "&x=1"},
		{"a@b.com----Gpws2uEk7F6gIMi7NlweDHt----NhBdTfK3pL9mQ2rV8sX1", "a@b.com----" + Mask + "----" + Mask},
		{"--api Gpws2uEk7F6gIMi7NlweDHt", "--api " + Mask},
		{"a@b.com----pass----JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP", "a@b.com----pass----" + Mask}, // a 2FA secret
		{"k7f2m9q4x8w3n6p1r5t0v2b8c4d6g9h3", Mask},
	} {
		if got, ok := Text(c.in); !ok || got != c.want {
			t.Errorf("Text(%q) = %q, %v; want %q", c.in, got, ok, c.want)
		}
	}
}

// Identifiers, model names, hashes and paths are not tokens.
func TestBareTokenFalsePositives(t *testing.T) {
	for _, in := range []string{
		"Base64EncodedStringV2", "TestHTTP2ServerPushHandler", "utf8ToUnicodeConverter",
		"Ipv4AddressResolverImpl", "MyAwesomeComponent2Props", "handleOAuth2CallbackRequest",
		"XMLHttpRequest2Polyfill", "SHA256WithRSAEncryption", "testAesGcm256EncryptDecrypt",
		"parseInt64FromStringOrZero", "AwsS3BucketPolicyV2Json", "isIPv6AddressInCIDRRange",
		"Qwen3-235B-A22B-Instruct-2507", "claude-opus-4-5-20251101", "gemini-2.5-flash-preview-05-20",
		"01M7SNESGW8K2Q4Z6X9B3C5D7F",                               // a ULID
		"9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822c",         // hex
		"/usr/local/lib/python3.12/site-packages/Abc1Def2Ghi3Jkl4", // a path
		"https://x.dev/v2/orders/Gpws2uEk7F6gIMi7NlweDHt",          // in a URL
		"user_id_Ab3dE5fG7hJ9kL1mN3pQ",                             // snake_case
		"x86_64-Ab3dE5fG7hJ9kL1mN3pQ-linux",                        // in a dashed name
		"v1.2.3+Ab3dE5fG7hJ9kL1mN3pQ",                              // in a version
		"THISISTHEDEFAULTCONFIGURATION2024", "deepseekv31terminus20250922release",
		"WINDOWS11PRO23H2X64ENGLISHINTL", "MACOSSEQUOIA1525B24F74DEVELOPERBETA",
		"ubuntu2404noblenumbatserverarm64img", "gpt4turbo20240409previewmodelv2release",
		`"files": "a\tb\t0ea1e91bbb3a60f729a8dc8f777fd2fc07cd8df4\n"`, // a SHA after \t
		"周会纪要：发版前检查",
	} {
		if got, ok := Text(in); ok {
			t.Errorf("Text(%q) masked: %q", in, got)
		}
	}
}

// TestBareTokenRecall measures how many random tokens are caught.
func TestBareTokenRecall(t *testing.T) {
	const (
		base62 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
		base32 = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	)
	r := rand.New(rand.NewPCG(1, 2))
	for _, c := range []struct {
		n   int
		min float64
		al  string
	}{{20, 0.88, base62}, {32, 0.95, base62}, {40, 0.97, base62}, {32, 0.75, base32}, {40, 0.85, base32}} {
		caught := 0
		const tries = 5000
		for range tries {
			b := make([]byte, c.n)
			for i := range b {
				b[i] = c.al[r.IntN(len(c.al))]
			}
			if bareToken(string(b)) {
				caught++
			}
		}
		if got := float64(caught) / tries; got < c.min {
			t.Errorf("%d characters of %q: caught %.1f%%, want at least %.0f%%", c.n, c.al, 100*got, 100*c.min)
		}
	}
}
