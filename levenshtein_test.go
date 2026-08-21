package levenshtein_test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	lev "github.com/yukiteruamano/fast-levenshtein/v2"
	agnivade "github.com/agnivade/levenshtein"
	arbovm "github.com/arbovm/levenshtein"
	dgryski "github.com/dgryski/trifles/leven"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func rndString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func rndStringArr(arrLen, strLen int) []string {
	b := make([]string, arrLen)
	for i := range b {
		b[i] = rndString(strLen)
	}
	return b
}

// ---------------------------------------------------------------------------
// Table-driven correctness tests (deterministic)
// ---------------------------------------------------------------------------

func TestDistanceTable(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "a", 1},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
		{"flaw", "lawn", 2},
		{"fast", "fastest", 3},
		{"abc", "abc", 0},
		{"abc", "def", 3},
		{"", "abc", 3},
		{"abc", "", 3},
		// Unicode — rune-based, not bytes
		{"café", "cafe", 1},
		{"café", "café", 0},
		{"😀", "😁", 1},
		{"a😀b", "a😁b", 1},
		{"hello😀", "hello", 1},
		// Combining / multi-byte
		{"résumé", "resume", 2},
		// Long strings crossing 64-rune boundary
		{string(make([]rune, 64)), string(make([]rune, 64)), 0},
		{string(make([]rune, 65)), string(make([]rune, 65)), 0},
		{string(make([]rune, 100)), string(append(make([]rune, 99), 'x')), 1},
		// Full Unicode outside BMP (U+1F600 onwards)
		{"\U0001F600\U0001F601\U0001F602", "\U0001F600\U0001F602", 1},
	}
	for _, tc := range cases {
		got := lev.Distance(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("Distance(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
		// Symmetry
		got2 := lev.Distance(tc.b, tc.a)
		if got2 != tc.want {
			t.Errorf("Distance symmetry (%q,%q)=%d want %d", tc.b, tc.a, got2, tc.want)
		}
		// Oracle vs agnivade for ASCII cases
		if len(tc.a) < 500 && len(tc.b) < 500 {
			// agnivade is rune-based and comparable for BMP; skip emoji oracle
			isASCII := func(s string) bool {
				for i := 0; i < len(s); i++ {
					if s[i] >= 128 {
						return false
					}
				}
				return true
			}
			if isASCII(tc.a) && isASCII(tc.b) {
				oracle := agnivade.ComputeDistance(tc.a, tc.b)
				if got != oracle {
					t.Errorf("oracle mismatch Distance(%q,%q)=%d agnivade=%d", tc.a, tc.b, got, oracle)
				}
			}
		}
	}
}

func TestDistanceWithCost(t *testing.T) {
	// Unit costs must equal Distance
	cases := [][2]string{
		{"kitten", "sitting"},
		{"flaw", "lawn"},
		{"café", "cafe"},
		{"😀😀", "😁😁"},
	}
	for _, c := range cases {
		a, b := c[0], c[1]
		got := lev.DistanceWithCost(a, b, lev.DefaultCost)
		want := lev.Distance(a, b)
		if got != want {
			t.Errorf("DistanceWithCost Default (%q,%q)=%d want %d", a, b, got, want)
		}
	}
	// Weighted costs: insert=2, delete=2, subst=1 vs subst=3
	// "ab" -> "ac": one substitution
	if got := lev.DistanceWithCost("ab", "ac", lev.Cost{Insert: 1, Delete: 1, Substitute: 2}); got != 2 {
		t.Errorf("weighted subst 2 got %d want 2", got)
	}
	if got := lev.DistanceWithCost("ab", "ac", lev.Cost{Insert: 10, Delete: 10, Substitute: 1}); got != 1 {
		t.Errorf("weighted subst cheap got %d want 1", got)
	}
	// Insert cost
	if got := lev.DistanceWithCost("a", "ab", lev.Cost{Insert: 2, Delete: 1, Substitute: 1}); got != 2 {
		t.Errorf("weighted insert got %d want 2", got)
	}
	// Delete cost
	if got := lev.DistanceWithCost("ab", "a", lev.Cost{Insert: 1, Delete: 3, Substitute: 1}); got != 3 {
		t.Errorf("weighted delete got %d want 3", got)
	}
	// Substitution more expensive than delete+insert
	if got := lev.DistanceWithCost("ab", "ac", lev.Cost{Insert: 1, Delete: 1, Substitute: 3}); got != 2 {
		t.Errorf("subst expensive (del+ins cheaper) got %d want 2", got)
	}
	// Empty strings with weighted costs
	if got := lev.DistanceWithCost("", "abc", lev.Cost{Insert: 2, Delete: 1, Substitute: 1}); got != 6 {
		t.Errorf("weighted empty insert got %d want 6", got)
	}
	if got := lev.DistanceWithCost("abc", "", lev.Cost{Insert: 1, Delete: 2, Substitute: 1}); got != 6 {
		t.Errorf("weighted empty delete got %d want 6", got)
	}
}

// Unicode-specific tests
func TestUnicode(t *testing.T) {
	tests := []struct{ a, b string; want int }{
		{"\U0001F600", "\U0001F600", 0},
		{"\U0001F600", "\U0001F601", 1},
		{"\U0001F600\U0001F600\U0001F600", "\U0001F600\U0001F600", 1},
		{"a\U0001F600b", "a\U0001F601b", 1},
		{"café", "cafe\u0301", 2}, // e + combining vs é precomposed are different runes
	}
	for _, tc := range tests {
		if got := lev.Distance(tc.a, tc.b); got != tc.want {
			t.Errorf("unicode Distance(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Concurrency — thread-safety
// ---------------------------------------------------------------------------

func TestConcurrent(t *testing.T) {
	const goroutines = 50
	const iterations = 1000
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errCh := make(chan string, goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				a := rndString(20 + rand.Intn(20))
				b := rndString(20 + rand.Intn(20))
				got := lev.Distance(a, b)
				want := agnivade.ComputeDistance(a, b)
				if got != want {
					errCh <- fmt.Sprintf("race Distance(%q,%q)=%d want %d", a, b, got, want)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for e := range errCh {
		t.Fatal(e)
	}
}

// ---------------------------------------------------------------------------
// Fuzz / oracle (100k random ASCII vs agnivade)
// ---------------------------------------------------------------------------

func TestFuzz(t *testing.T) {
	for i := 0; i < 20000; i++ {
		str1 := rndString(rand.Intn(200))
		str2 := rndString(rand.Intn(200))
		re1 := lev.Distance(str1, str2)
		re2 := agnivade.ComputeDistance(str1, str2)
		if re1 != re2 {
			t.Errorf("TestFuzz[%d]: Distance(%q,%q)=%d agnivade=%d", i, str1, str2, re1, re2)
		}
	}
}

// Go 1.18+ native fuzz target
func FuzzDistance(f *testing.F) {
	f.Add("kitten", "sitting")
	f.Add("", "")
	f.Add("café", "cafe")
	f.Add("😀", "😁")
	f.Fuzz(func(t *testing.T, a, b string) {
		got := lev.Distance(a, b)
		if got < 0 {
			t.Fatalf("negative distance %d for %q %q", got, a, b)
		}
		// Symmetry
		if got2 := lev.Distance(b, a); got != got2 {
			t.Fatalf("asymmetry %q %q: %d != %d", a, b, got, got2)
		}
		// Empty
		if a == "" || b == "" {
			// Distance to empty is rune count of other
			want := 0
			if a == "" {
				want = len([]rune(b))
			} else {
				want = len([]rune(a))
			}
			if got != want {
				t.Fatalf("empty distance %q %q: got %d want %d", a, b, got, want)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func Benchmark(b *testing.B) {
	data := [...][]string{
		rndStringArr(500, 4),
		rndStringArr(500, 8),
		rndStringArr(500, 16),
		rndStringArr(500, 32),
		rndStringArr(500, 64),
		rndStringArr(500, 128),
		rndStringArr(500, 256),
		rndStringArr(500, 512),
		rndStringArr(500, 1024),
	}
	tmp := 0
	for i, pick := range data {
		b.Run(fmt.Sprint(1<<(i+2)), func(b *testing.B) {
			b.Run("yukiteruamano", func(b *testing.B) {
				b.ReportAllocs()
				for n := 0; n < b.N; n++ {
					for j := 0; j < len(pick)-1; j++ {
						tmp += lev.Distance(pick[j], pick[j+1])
					}
				}
			})
			b.Run("agniva", func(b *testing.B) {
				b.ReportAllocs()
				for n := 0; n < b.N; n++ {
					for j := 0; j < len(pick)-1; j++ {
						tmp += agnivade.ComputeDistance(pick[j], pick[j+1])
					}
				}
			})
			b.Run("arbovm", func(b *testing.B) {
				b.ReportAllocs()
				for n := 0; n < b.N; n++ {
					for j := 0; j < len(pick)-1; j++ {
						tmp += arbovm.Distance(pick[j], pick[j+1])
					}
				}
			})
			b.Run("dgryski", func(b *testing.B) {
				b.ReportAllocs()
				for n := 0; n < b.N; n++ {
					for j := 0; j < len(pick)-1; j++ {
						tmp += dgryski.Levenshtein([]rune(pick[j]), []rune(pick[j+1]))
					}
				}
			})
		})
	}
	// Prevent optimization
	if tmp == 0 {
		b.Log(tmp)
	}
}

func BenchmarkDistanceWithCost(b *testing.B) {
	pairs := rndStringArr(1000, 32)
	cost := lev.Cost{Insert: 2, Delete: 2, Substitute: 3}
	b.ReportAllocs()
	b.ResetTimer()
	sum := 0
	for i := 0; i < b.N; i++ {
		for j := 0; j < len(pairs)-1; j++ {
			sum += lev.DistanceWithCost(pairs[j], pairs[j+1], cost)
		}
	}
	if sum == 0 {
		b.Log(sum)
	}
}
