// Package levenshtein provides fast Levenshtein distance calculation.
//
// This is a thread-safe, full-Unicode implementation based on Myers'
// bit-parallel algorithm. It supports the full Unicode range
// (U+0000..U+10FFFF) and is safe for concurrent use.
//
// Two execution paths are provided:
//   - m64ASCII / m64Unicode: pattern <= 64 runes, O(|text|) bit-parallel.
//   - mxASCII / mxUnicode: blocked Myers for arbitrary lengths.
//
// An ASCII fast-path uses a stack-allocated [256]uint64 table (2 KiB)
// instead of a map for minimal allocations.
package levenshtein



// Cost defines weighted edit costs for DistanceWithCost.
type Cost struct {
	Insert     int // cost to insert a rune
	Delete     int // cost to delete a rune
	Substitute int // cost to substitute a rune
}

// DefaultCost is the standard unit cost (insert=delete=subst=1).
var DefaultCost = Cost{Insert: 1, Delete: 1, Substitute: 1}

// countRunesAndASCII returns rune count and whether s is ASCII-only in one pass.
func countRunesAndASCII(s string) (int, bool) {
	n := 0
	ascii := true
	for _, r := range s {
		n++
		if r >= 128 {
			ascii = false
		}
	}
	return n, ascii
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// Distance returns the Levenshtein distance between a and b measured in
// runes (Unicode code points). It is thread-safe and supports the full
// Unicode range.
//
// It is equivalent to DistanceWithCost(a, b, DefaultCost).
func Distance(a, b string) int {
	return DistanceWithCost(a, b, DefaultCost)
}

// DistanceWithCost returns the weighted Levenshtein distance between a and b.
// Costs must be >= 0. When c equals DefaultCost the fast Myers
// bit-parallel path is used; otherwise a two-row DP with O(n*m) time and
// O(min(n,m)) memory is used.
func DistanceWithCost(a, b string, c Cost) int {
	if c.Insert < 0 || c.Delete < 0 || c.Substitute < 0 {
		// Negative costs are not meaningful; treat as 0.
		if c.Insert < 0 {
			c.Insert = 0
		}
		if c.Delete < 0 {
			c.Delete = 0
		}
		if c.Substitute < 0 {
			c.Substitute = 0
		}
	}
	// Fast path for unit costs.
	if c.Insert == 1 && c.Delete == 1 && c.Substitute == 1 {
		return fastDistance(a, b)
	}
	return weightedDistance(a, b, c)
}

// fastDistance is the Myers bit-parallel path (unit cost).
func fastDistance(a, b string) int {
	// Work in runes, not bytes. Single pass per string for count+ASCII.
	ra, asciiA := countRunesAndASCII(a)
	rb, asciiB := countRunesAndASCII(b)
	if ra < rb {
		a, b = b, a
		ra, rb = rb, ra
		asciiA, asciiB = asciiB, asciiA
	}
	if rb == 0 {
		return ra
	}
	bothASCII := asciiA && asciiB
	if ra <= 64 {
		if bothASCII {
			return m64ASCII(a, b)
		}
		return m64Unicode(a, b)
	}
	if bothASCII {
		return mxASCII(a, b)
	}
	return mxUnicode(a, b)
}

// weightedDistance computes Levenshtein with arbitrary costs using
// two-row DP. Memory O(min(n,m)).
func weightedDistance(a, b string, c Cost) int {
	ar := []rune(a)
	br := []rune(b)
	n := len(ar)
	m := len(br)
	if n == 0 {
		return m * c.Insert
	}
	if m == 0 {
		return n * c.Delete
	}
	// Ensure br is the shorter to minimize allocations.
	if m > n {
		ar, br = br, ar
		n, m = m, n
		// Swap insert/delete when strings are swapped.
		c.Insert, c.Delete = c.Delete, c.Insert
	}
	prev := make([]int, m+1)
	cur := make([]int, m+1)
	for j := 0; j <= m; j++ {
		prev[j] = j * c.Insert
	}
	for i := 1; i <= n; i++ {
		cur[0] = i * c.Delete
		for j := 1; j <= m; j++ {
			substCost := 0
			if ar[i-1] != br[j-1] {
				substCost = c.Substitute
			}
			del := prev[j] + c.Delete
			ins := cur[j-1] + c.Insert
			sub := prev[j-1] + substCost
			// Classic min of three, but substitution can be cheaper
			// than delete+insert if c.Substitute < c.Insert+c.Delete.
			// Direct min is correct.
			min := del
			if ins < min {
				min = ins
			}
			if sub < min {
				min = sub
			}
			cur[j] = min
		}
		prev, cur = cur, prev
	}
	return prev[m]
}

// ---------------------------------------------------------------------------
// Myers bit-parallel — ASCII fast-path (m64ASCII/mxASCII) vs Unicode
// ---------------------------------------------------------------------------

// m64ASCII uses a stack array [256]uint64 for ASCII.
func m64ASCII(a, b string) int {
	var peq [256]uint64
	sc := 0
	for _, ch := range a {
		peq[byte(ch)] |= 1 << sc
		sc++
	}
	ls := uint64(1) << (sc - 1)
	pv := ^uint64(0)
	mv := uint64(0)
	score := sc
	for _, ch := range b {
		eq := peq[byte(ch)]
		xv := eq | mv
		eq |= ((eq & pv) + pv) ^ pv
		mv |= ^(eq | pv)
		pv &= eq
		if mv&ls != 0 {
			score++
		}
		if pv&ls != 0 {
			score--
		}
		mv = (mv << 1) | 1
		pv = (pv << 1) | ^(xv | mv)
		mv &= xv
	}
	return score
}

// m64Unicode uses a map for full Unicode (thread-safe, local).
func m64Unicode(a, b string) int {
	peq := make(map[rune]uint64, 64)
	sc := 0
	for _, ch := range a {
		peq[ch] |= 1 << sc
		sc++
	}
	ls := uint64(1) << (sc - 1)
	pv := ^uint64(0)
	mv := uint64(0)
	score := sc
	for _, ch := range b {
		eq := peq[ch]
		xv := eq | mv
		eq |= ((eq & pv) + pv) ^ pv
		mv |= ^(eq | pv)
		pv &= eq
		if mv&ls != 0 {
			score++
		}
		if pv&ls != 0 {
			score--
		}
		mv = (mv << 1) | 1
		pv = (pv << 1) | ^(xv | mv)
		mv &= xv
	}
	return score
}

// mxASCII is the blocked Myers implementation for ASCII using array.
func mxASCII(a, b string) int {
	s1 := []rune(a)
	s2 := []rune(b)
	n := len(s1)
	m := len(s2)
	hsize := 1 + ((n - 1) / 64)
	vsize := 1 + ((m - 1) / 64)
	phc := make([]uint64, hsize)
	mhc := make([]uint64, hsize)
	for i := 0; i < hsize; i++ {
		phc[i] = ^uint64(0)
		mhc[i] = 0
	}
	var peq [256]uint64
	j := 0
	for ; j < vsize-1; j++ {
		mv := uint64(0)
		pv := ^uint64(0)
		start := j * 64
		vlen := min(64, m-start) + start
		for k := start; k < vlen; k++ {
			peq[byte(s2[k])] |= 1 << (k & 63)
		}
		for i := 0; i < n; i++ {
			eq := peq[byte(s1[i])]
			pb := (phc[i/64] >> (i & 63)) & 1
			mb := (mhc[i/64] >> (i & 63)) & 1
			xv := eq | mv
			xh := ((((eq | mb) & pv) + pv) ^ pv) | eq | mb
			ph := mv | ^(xh | pv)
			mh := pv & xh
			if ((ph >> 63) ^ pb) != 0 {
				phc[i/64] ^= 1 << (i & 63)
			}
			if ((mh >> 63) ^ mb) != 0 {
				mhc[i/64] ^= 1 << (i & 63)
			}
			ph = (ph << 1) | pb
			mh = (mh << 1) | mb
			pv = mh | ^(xv | ph)
			mv = ph & xv
		}
		for k := start; k < vlen; k++ {
			peq[byte(s2[k])] = 0
		}
	}
	mv := uint64(0)
	pv := ^uint64(0)
	start := j * 64
	vlen := min(64, m-start) + start
	for k := start; k < vlen; k++ {
		peq[byte(s2[k])] |= 1 << (k & 63)
	}
	sc := uint64(m)
	for i := 0; i < n; i++ {
		eq := peq[byte(s1[i])]
		pb := (phc[i/64] >> (i & 63)) & 1
		mb := (mhc[i/64] >> (i & 63)) & 1
		xv := eq | mv
		xh := ((((eq | mb) & pv) + pv) ^ pv) | eq | mb
		ph := mv | ^(xh | pv)
		mh := pv & xh
		sc += (ph >> ((m - 1) & 63)) & 1
		sc -= (mh >> ((m - 1) & 63)) & 1
		if ((ph >> 63) ^ pb) != 0 {
			phc[i/64] ^= 1 << (i & 63)
		}
		if ((mh >> 63) ^ mb) != 0 {
			mhc[i/64] ^= 1 << (i & 63)
		}
		ph = (ph << 1) | pb
		mh = (mh << 1) | mb
		pv = mh | ^(xv | ph)
		mv = ph & xv
	}
	return int(sc)
}

// mxUnicode is the blocked Myers implementation for full Unicode using map.
func mxUnicode(a, b string) int {
	s1 := []rune(a)
	s2 := []rune(b)
	n := len(s1)
	m := len(s2)
	hsize := 1 + ((n - 1) / 64)
	vsize := 1 + ((m - 1) / 64)
	phc := make([]uint64, hsize)
	mhc := make([]uint64, hsize)
	for i := 0; i < hsize; i++ {
		phc[i] = ^uint64(0)
		mhc[i] = 0
	}
	peq := make(map[rune]uint64, 64)
	j := 0
	for ; j < vsize-1; j++ {
		mv := uint64(0)
		pv := ^uint64(0)
		start := j * 64
		vlen := min(64, m-start) + start
		for k := start; k < vlen; k++ {
			peq[s2[k]] |= 1 << (k & 63)
		}
		for i := 0; i < n; i++ {
			eq := peq[s1[i]]
			pb := (phc[i/64] >> (i & 63)) & 1
			mb := (mhc[i/64] >> (i & 63)) & 1
			xv := eq | mv
			xh := ((((eq | mb) & pv) + pv) ^ pv) | eq | mb
			ph := mv | ^(xh | pv)
			mh := pv & xh
			if ((ph >> 63) ^ pb) != 0 {
				phc[i/64] ^= 1 << (i & 63)
			}
			if ((mh >> 63) ^ mb) != 0 {
				mhc[i/64] ^= 1 << (i & 63)
			}
			ph = (ph << 1) | pb
			mh = (mh << 1) | mb
			pv = mh | ^(xv | ph)
			mv = ph & xv
		}
		for k := start; k < vlen; k++ {
			delete(peq, s2[k])
		}
	}
	mv := uint64(0)
	pv := ^uint64(0)
	start := j * 64
	vlen := min(64, m-start) + start
	for k := start; k < vlen; k++ {
		peq[s2[k]] |= 1 << (k & 63)
	}
	sc := uint64(m)
	for i := 0; i < n; i++ {
		eq := peq[s1[i]]
		pb := (phc[i/64] >> (i & 63)) & 1
		mb := (mhc[i/64] >> (i & 63)) & 1
		xv := eq | mv
		xh := ((((eq | mb) & pv) + pv) ^ pv) | eq | mb
		ph := mv | ^(xh | pv)
		mh := pv & xh
		sc += (ph >> ((m - 1) & 63)) & 1
		sc -= (mh >> ((m - 1) & 63)) & 1
		if ((ph >> 63) ^ pb) != 0 {
			phc[i/64] ^= 1 << (i & 63)
		}
		if ((mh >> 63) ^ mb) != 0 {
			mhc[i/64] ^= 1 << (i & 63)
		}
		ph = (ph << 1) | pb
		mh = (mh << 1) | mb
		pv = mh | ^(xv | ph)
		mv = ph & xv
	}
	return int(sc)
}
