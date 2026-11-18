package todo

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	got := Parse("- [ ] a\r\n- [x] b\n\n* [X] c\nplain line\n- [ ]\n")
	want := []Item{{"a", false}, {"b", true}, {"c", true}, {"plain line", false}, {"", false}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
	if p := Count(got); p.Done != 2 || p.Total != 5 {
		t.Fatalf("progress %+v", p)
	}
}

// Canonical files round-trip byte for byte.
func TestRoundTrip(t *testing.T) {
	files, _ := filepath.Glob("testdata/*.txt")
	if len(files) == 0 {
		t.Fatal("no testdata")
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		in := strings.TrimSuffix(string(b), "\n")
		if out := Format(Parse(in)); out != in {
			t.Errorf("%s: round trip\n got %q\nwant %q", f, out, in)
		}
	}
}
