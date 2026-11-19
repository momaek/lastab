package title

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/momaek/lastab/internal/redact"
)

func TestMake(t *testing.T) {
	cases := []struct {
		kind Kind
		body string
		want string
	}{
		{Text, "curl -X POST 'https://api.staging.example.com/v2/orders/search' \\\n  -H 'Authorization: Bearer abcdefgh12345678'", "curl · POST /v2/orders/search"},
		{Text, "curl https://example.com/health", "curl · GET /health"},
		{Text, "curl -d 'a=1' https://example.com/login", "curl · POST /login"},
		{Text, `{"status":"paid","order_id":"A-20391","items":[1,2]}`, "JSON · order_id A-20391"},
		{Text, `[{"name":"lastab"}]`, "JSON · name lastab"},
		{Todo, "- [ ] 发版前检查\n- [x] 跑测试\n- [x] 打 tag\n", "发版前检查 · 2/3"},
		{Text, "{\n\n  周会纪要 第 3 周\nmore", "周会纪要 第 3 周"},
		{Text, "   \n\t\n", ""},
		{Text, "Authorization: Bearer abcdefgh12345678", "Authorization: Bearer ••••••••"},
	}
	for _, c := range cases {
		if got := Make(c.kind, c.body); got != c.want {
			t.Errorf("Make(%q) = %q; want %q", c.body, got, c.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	got := Make(Text, strings.Repeat("长", 100))
	if utf8.RuneCountInString(got) != MaxRunes || !strings.HasSuffix(got, "…") {
		t.Errorf("got %q", got)
	}
}

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"  staging\n orders\t repro ":            "staging orders repro",
		"\x1b[31mred\x1b[0m":                     "[31mred [0m",
		" \n\t ":                                 "",
		"Authorization: Bearer abcdefgh12345678": "Authorization: Bearer " + redact.Mask,
		strings.Repeat("长", 50):                  strings.Repeat("长", MaxRunes-1) + "…",
	} {
		if got := Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
}
