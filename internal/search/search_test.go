package search

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/momaek/lastab/internal/store/storetest"
)

func TestPreviewHighlights(t *testing.T) {
	body := strings.Repeat("filler line\n", 30) + "the Order_ID is A-20391 and order_id again"
	p, hs := Preview(body, "order_id", 80)
	if !strings.HasPrefix(p, "…") {
		t.Errorf("preview should start mid-text: %q", p)
	}
	if len(hs) != 2 {
		t.Fatalf("highlights %v in %q", hs, p)
	}
	for _, h := range hs {
		if !strings.EqualFold(p[h.Start:h.End], "order_id") {
			t.Errorf("highlight %q", p[h.Start:h.End])
		}
	}
}

func TestPreviewMasks(t *testing.T) {
	p, _ := Preview("curl -H 'Authorization: Bearer abcdefgh12345678'", "", 200)
	if strings.Contains(p, "abcdefgh") {
		t.Errorf("secret in preview: %q", p)
	}
}

func TestPreviewCJK(t *testing.T) {
	body := strings.Repeat("中文内容", 50) + "周会纪要"
	p, hs := Preview(body, "周会", 60)
	if len(hs) != 1 || p[hs[0].Start:hs[0].End] != "周会" {
		t.Fatalf("%q %v", p, hs)
	}
}

func TestMatches(t *testing.T) {
	if !Matches("curl · POST", "", "post") || Matches("a", "b", "zz") || !Matches("", "", " ") {
		t.Error("Matches")
	}
}

func TestHighlights(t *testing.T) {
	hs := Highlights("Staging and staging-b", "staging")
	if len(hs) != 2 || hs[1].Start != 12 {
		t.Fatalf("%v", hs)
	}
	if Highlights("abc", " ") != nil {
		t.Error("blank query")
	}
}

// TestPreviewWindow checks that masking a window around the match gives
// the same preview as masking the whole body, over generated snippets and
// bodies with secrets near the window's edges.
func TestPreviewWindow(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 7))
	var bodies []string
	for range 500 {
		_, b := storetest.Body(r)
		bodies = append(bodies, b)
	}
	filler := strings.Repeat("一行普通的文字 plain filler text\n", 40)
	jwt := "eyJhbGciOiJIUzI1NiJ9." + strings.Repeat("eyJzdWIiOiIxMjM0NTY3ODkwIn0", 20) + ".c2lnbmF0dXJlc2lnbmF0dXJl"
	bodies = append(bodies,
		filler+"Authorization:\n  Bearer abcdefgh12345678\n"+filler+"needle",
		filler+`"password":`+"\n"+`  "hunter2hunter2"`+"\nneedle "+filler,
		"needle\n"+strings.Repeat("token="+jwt+"\n", 30)+"needle at the end",
		strings.Repeat("x", 5000)+" api_key=abcd1234efgh needle "+strings.Repeat("y", 5000),
		filler+"-----BEGIN RSA PRIVATE KEY-----\nMIIEow\n"+filler+"needle\n-----END RSA PRIVATE KEY-----\n",
		"needle sk-ant-"+strings.Repeat("a", 40)+" needle",
		"sk-ant-needle"+strings.Repeat("a", 40)+"\nneedle",
		"",
		"needle",
	)
	queries := []string{"", "needle", "周会", "数据库", "the", "--since", `{"`, "Bearer", "sk_test", "token", "error", "x", "a", storetest.Absent}
	bad := 0
	for _, b := range bodies {
		for _, q := range queries {
			got, gotHs := Preview(b, q, 160)
			want, wantHs := previewFull(b, q, 160)
			if got != want || len(gotHs) != len(wantHs) {
				if bad++; bad <= 5 {
					t.Errorf("Preview(%.60q, %q):\n got %q\nwant %q", b, q, got, want)
				}
			}
		}
	}
	if bad > 0 {
		t.Errorf("%d previews differ", bad)
	}
}
