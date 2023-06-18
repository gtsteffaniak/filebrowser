# natural

Yet another natural sort, with 100% test coverage and a benchmark. It **does not
allocate memory**, doesn't depend on package `sort` and hence doesn't depend on
`reflect`. It is optimized for speed.

[![Go
Reference](https://pkg.go.dev/badge/github.com/maruel/natural.svg)](https://pkg.go.dev/github.com/maruel/natural)
[![codecov](https://codecov.io/gh/maruel/natural/branch/main/graph/badge.svg?token=iQg8Y62BBg)](https://codecov.io/gh/maruel/natural)


## Benchmarks

On Go 1.18.3.

```
$ go test -bench=. -cpu 1
goos: linux
goarch: amd64
pkg: github.com/maruel/natural
cpu: Intel(R) Core(TM) i7-10700 CPU @ 2.90GHz
BenchmarkLessDigitsTwoGroupsNative 331287298     3.597 ns/op   0 B/op   0 allocs/op
BenchmarkLessDigitsTwoGroups        32479050    36.55 ns/op    0 B/op   0 allocs/op
BenchmarkLessStringOnly            157775884     7.603 ns/op   0 B/op   0 allocs/op
BenchmarkLessDigitsOnly             69210796    17.52 ns/op    0 B/op   0 allocs/op
BenchmarkLess10Blocks                6331066   190.8 ns/op     0 B/op   0 allocs/op
```

On a Raspberry Pi 3:

```
$ go test -bench=. -cpu 1
goos: linux
goarch: arm
pkg: github.com/maruel/natural
BenchmarkLessDigitsTwoGroupsNative  14181789    86.57 ns/op    0 B/op   0 allocs/op
BenchmarkLessDigitsTwoGroups         1600195   748.9 ns/op     0 B/op   0 allocs/op
BenchmarkLessStringOnly              8286034   142.3 ns/op     0 B/op   0 allocs/op
BenchmarkLessDigitsOnly              3653055   331.4 ns/op     0 B/op   0 allocs/op
BenchmarkLess10Blocks                 310687  3838 ns/op       0 B/op   0 allocs/op
```

Coverage:

```
$ go test -cover
PASS
coverage: 100.0% of statements
ok     github.com/maruel/natural       0.012s
```
