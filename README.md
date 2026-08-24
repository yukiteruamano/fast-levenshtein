# fast-levenshtein :rocket:

[![CI](https://github.com/yukiteruamano/fast-levenshtein/actions/workflows/ci.yml/badge.svg)](https://github.com/yukiteruamano/fast-levenshtein/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/yukiteruamano/fast-levenshtein/v2.svg)](https://pkg.go.dev/github.com/yukiteruamano/fast-levenshtein/v2)
[![Go Version](https://img.shields.io/badge/go-1.26%2B-blue)](go.mod)

> Fastest Levenshtein implementation in Go — now thread-safe, full-Unicode, and Go 1.26.

Measure the difference between two strings in runes (Unicode code points).

Originally by [ka-weihe](https://github.com/ka-weihe/fast-levenshtein), maintained and modernized by [yukiteruamano](https://github.com/yukiteruamano/fast-levenshtein).

## Features

- **Thread-safe** — no global state; safe for concurrent use (`go test -race` clean)
- **Full Unicode** — supports `U+0000..U+10FFFF` (including emoji, CJK extensions); distance is measured in runes, not bytes
- **Fast paths** — ASCII-only strings use a stack-allocated `[256]uint64` table (2 KiB, 0 heap allocs for `<=64` runes); Unicode uses a local `map[rune]uint64`
- **Myers bit-parallel** — `O((n/64)*m)` blocked Myers for arbitrary lengths
- **Weighted API** — `DistanceWithCost` for custom insert/delete/substitute costs
- **Go 1.26** (`toolchain go1.26.5`), module `github.com/yukiteruamano/fast-levenshtein/v2`

## Installation

```bash
go get github.com/yukiteruamano/fast-levenshtein/v2
```

Requires Go 1.26+.

## Usage

```go
package main

import (
	"fmt"
	lev "github.com/yukiteruamano/fast-levenshtein/v2"
)

func main() {
	// Basic (unit cost)
	fmt.Println(lev.Distance("kitten", "sitting")) // 3
	fmt.Println(lev.Distance("café", "cafe"))       // 1
	fmt.Println(lev.Distance("😀", "😁"))           // 1
	fmt.Println(lev.Distance("fast", "fastest"))    // 3

	// Weighted — insertion=2, deletion=2, substitution=1
	cost := lev.Cost{Insert: 2, Delete: 2, Substitute: 1}
	fmt.Println(lev.DistanceWithCost("ab", "ac", cost)) // 1
}
```

### API

```go
func Distance(a, b string) int
func DistanceWithCost(a, b string, c Cost) int

type Cost struct {
	Insert     int
	Delete     int
	Substitute int
}
var DefaultCost = Cost{Insert: 1, Delete: 1, Substitute: 1}
```

- `Distance` is equivalent to `DistanceWithCost(a, b, DefaultCost)` and uses the fast Myers path.
- `DistanceWithCost` with non-unit costs falls back to a two-row DP (`O(n*m)` time, `O(min(n,m))` memory). Substitution can be cheaper than `Delete+Insert`; the minimum is taken correctly.

## Correctness

- Distance is computed on **runes**, not bytes. `"café"` (4 runes, 5 bytes) vs `"cafe"` (4 runes) is `1`, not `2`.
- Handles empty strings, equal strings, and any length (tested beyond 1024 runes).
- Full Unicode including outside BMP (e.g. `U+1F600` emoji) — no `0x10000` limit.
- Symmetric: `Distance(a,b) == Distance(b,a)`.

## Concurrency

No global mutable state. The former `var peq [0x10000]uint64` has been replaced by function-local tables. Safe to call from any goroutine.

```go
var wg sync.WaitGroup
for i:=0; i<100; i++ {
    wg.Add(1)
    go func() {
        lev.Distance(a, b)
        wg.Done()
    }()
}
wg.Wait()
```

## Benchmarks

Run with Go 1.26.5, `go test -bench=. -benchmem`:

`yukiteruamano` is this package (v2). Bench compares vs `agnivade/levenshtein`, `arbovm/levenshtein`, `dgryski/trifles`.

```
# ASCII, 500 pairs each, Go 1.26.5 linux/amd64
Benchmark/4/yukiteruamano-12     < ~13k ns/op   0 B/op  0 allocs/op
Benchmark/8/yukiteruamano-12     < ~23k ns/op   0 B/op  0 allocs/op
Benchmark/16/yukiteruamano-12    < ~44k ns/op   0 B/op  0 allocs/op
Benchmark/32/yukiteruamano-12    < ~94k ns/op   0 B/op  0 allocs/op
Benchmark/64/yukiteruamano-12    < ~180k ns/op  0 B/op  0 allocs/op  (Myers m64 fast path)
Benchmark/128/yukiteruamano-12   (blocked Myers, see go test -bench)
Benchmark/1024/yukiteruamano-12  (blocked Myers, 0-1 allocs/op for Unicode map)
```

Legacy numbers from the original README (Go 1.15, ka-weihe) for reference — 15x faster for length 64 vs second fastest:

```
Benchmark/64/kaweihe-12    6538    180181 ns/op  0 B/op  0 allocs/op
Benchmark/64/agniva-12      422   2827182 ns/op  327344 B/op  1497 allocs/op
```

Current v2 retains the 0-alloc fast path for ASCII `<=64` runes and remains thread-safe.

Run locally:

```bash
go test -bench=. -benchmem -count=5
go test -bench=BenchmarkDistanceWithCost -benchmem
```

## Testing

```bash
go vet ./...
go test -race -count=1 ./...
go test -cover ./...
go test -fuzz FuzzDistance -fuzztime=10s
```

Covered: table-driven unit tests, Unicode (BMP + outside BMP), weighted costs, concurrency (`TestConcurrent`), 20k random oracle vs `agnivade`, and native `FuzzDistance`.

## Migration from ka-weihe

```bash
# old
go get github.com/ka-weihe/fast-levenshtein
import "github.com/ka-weihe/fast-levenshtein"

# new (v2, Go 1.26+)
go get github.com/yukiteruamano/fast-levenshtein/v2
import lev "github.com/yukiteruamano/fast-levenshtein/v2"
```

API is backward-compatible for `Distance(s1,s2) int`. Behavior change: distance is now rune-based (correct Unicode) — multi-byte strings that previously returned byte-length-based values will now return the proper rune distance.

## Development & Release

### Makefile

```bash
make help          # lista de targets
make ci            # vet + race + cover (gate local, igual que CI)
make test          # go test ./...
make cover         # cobertura + HTML
make bench         # benchmarks
make fuzz          # fuzz FuzzDistance 30s
```

### Versionado CalVer (`vYYYY.MM.DD`)

El proyecto usa **CalVer basado en fecha** `v2026.08.24` replicado en:

* **Git tags:** `v2.20260824.0` (canónico semver para módulo `/v2`, requerido por `proxy.golang.org`) + alias `v2026.08.24` legible. El canónico es `v2.YYYYMMDD.0` porque `v2.2026.08.24` (4 partes) es semver inválido y el proxy responde 404.
* **pkg.go.dev:** `https://pkg.go.dev/github.com/yukiteruamano/fast-levenshtein/v2@v2.20260824.0` se indexa automáticamente al pushear el tag.

Si hay 2 releases el mismo día, se gera sufijo `.1`: `v2.20260824.1` + alias `v2026.08.24.1`.

```bash
make version              # muestra VERSION/TAG calculados (ej. v2.20260824.0)
make version VERSION=2026.08.25  # override fecha -> v2.20260825.0
```

### Flujo de publicación

```bash
# 1. Commitea tus cambios
git add -A && git commit -m "feat: ..."

# 2. Push de commits (sin tags)
make push                 # git push origin main

# 3. Crear tag fecha + push + publicar en pkg.go.dev (requiere tests OK)
make tags                 # crea v2.20260824.0 + v2026.08.24, push y GOPROXY refresh (con reintentos)
# o fecha específica:
make tags VERSION=2026.08.25  # -> v2.20260825.0 + v2026.08.25

# Flujo completo en uno (ci + tags):
make release

# Refrescar manualmente un tag ya existente en pkg.go.dev:
make publish TAG=v2.20260824.0
```

`pkg.go.dev` no tiene "upload" manual: se indexa al hacer `GOPROXY=proxy.golang.org go list -m github.com/yukiteruamano/fast-levenshtein/v2@v2.20260824.0`, que es lo que hace `make publish` y el workflow `release.yml` automáticamente. El Makefile reintenta 5x con delay porque el proxy puede tardar 10-30s tras el push.

Verifica:

```bash
curl -s https://proxy.golang.org/github.com/yukiteruamano/fast-levenshtein/v2/@v/v2.20260824.0.info
curl -s https://proxy.golang.org/github.com/yukiteruamano/fast-levenshtein/v2/@v/list | tr ',' '\n' | grep 2026
go list -m -json github.com/yukiteruamano/fast-levenshtein/v2@v2.20260824.0
```

### CI/CD

* **`.github/workflows/ci.yml`**: en cada `push`/`PR` a `main` corre `go vet`, `go test -race`, `coverage`, `fuzz 10s` y `golangci-lint` en matrix Go 1.26/1.27.
* **`.github/workflows/release.yml`**: al pushear tag `v*` (o manual `workflow_dispatch`) valida y ejecuta `go list -m` contra `proxy.golang.org` para indexar `pkg.go.dev`.

## License

MIT — see [LICENSE](LICENSE). Copyright (c) 2020 ka-weihe, Copyright (c) 2026 yukiteruamano.
