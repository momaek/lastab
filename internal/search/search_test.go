package search

import (
	"strings"
	"testing"
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
