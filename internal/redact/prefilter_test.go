package redact_test

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/momaek/lastab/internal/redact"
	"github.com/momaek/lastab/internal/store/storetest"
)

// TestPrefilter checks that skipping rules by their literals masks exactly
// what running every rule does.
func TestPrefilter(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 3))
	in := []string{
		"-H 'AUTHORIZATION: BEARER abcdef0123456789'",
		"ſecret=hunter22hunter", // long s folds to s under (?i)
		"api_Key: abcd1234efgh", // Kelvin sign folds to k
		`{"Client-Secret": "abcd1234"}`,
		"PASSWD => s3cr3tvalue",
		"x-Api-Token: abcdefgh",
		"-----BEGIN EC PRIVATE KEY-----\nabc\n-----END EC PRIVATE KEY-----",
		"github_pat_" + strings.Repeat("a", 30),
		"xoxb-1234567890-abc",
		"ASIAABCDEFGHIJKLMNOP",
		"AIza" + strings.Repeat("b", 35),
	}
	for range 2000 {
		_, b := storetest.Body(r)
		in = append(in, b)
	}
	for _, s := range in {
		got, gotOK := redact.Text(s)
		want, wantOK := redact.TextAll(s)
		if got != want || gotOK != wantOK {
			t.Errorf("Text(%.80q) = %.80q, %v; every rule gives %.80q, %v", s, got, gotOK, want, wantOK)
		}
	}
}
