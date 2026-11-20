package redact_test

import (
	"math/rand/v2"
	"testing"

	"github.com/momaek/lastab/internal/redact"
	"github.com/momaek/lastab/internal/store/storetest"
)

// BenchmarkText masks a realistic mix of snippet bodies.
func BenchmarkText(b *testing.B) {
	r := rand.New(rand.NewPCG(7, 7))
	var bodies []string
	n := 0
	for range 200 {
		_, s := storetest.Body(r)
		bodies = append(bodies, s)
		n += len(s)
	}
	b.SetBytes(int64(n))
	for b.Loop() {
		for _, s := range bodies {
			redact.Text(s)
		}
	}
}
