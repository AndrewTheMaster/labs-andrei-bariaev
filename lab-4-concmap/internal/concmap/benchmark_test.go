package concmap

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// Бенчи читают BENCH_KEYS=4096,65536 и необязательно BENCH_BUCKET_BITS (default 12).

func envSizes(tb testing.TB) []int {
	raw := strings.TrimSpace(os.Getenv("BENCH_KEYS"))
	if raw == "" {
		return []int{4096, 65536}
	}
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || n <= 0 {
			tb.Fatalf("неверный элемент BENCH_KEYS: %q", p)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		tb.Fatal("пустой BENCH_KEYS после разбора")
	}
	return out
}

func envBucketBits(tb testing.TB) uint8 {
	s := strings.TrimSpace(os.Getenv("BENCH_BUCKET_BITS"))
	if s == "" {
		return 12
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 4 || n > 26 {
		tb.Fatalf("BENCH_BUCKET_BITS must be [4..26], got %q", s)
	}
	return uint8(n)
}

func makeKeys(sz int) []string {
	keys := make([]string, sz)
	for i := 0; i < sz; i++ {
		keys[i] = fmt.Sprintf("k_%d", i)
	}
	return keys
}

func fillConc(keys []string, bits uint8) *Map[string, int] {
	m := New[string, int](bits)
	for i, k := range keys {
		m.Put(k, i)
	}
	return m
}

func fillPlain(keys []string) *Plain[string, int] {
	m := NewPlain[string, int]()
	for i, k := range keys {
		m.Put(k, i)
	}
	return m
}

func BenchmarkParallelGetHit(b *testing.B) {
	sizes := envSizes(b)
	bbits := envBucketBits(b)

	for _, sz := range sizes {
		keys := makeKeys(sz)
		b.Run(fmt.Sprintf("size_%d", sz), func(b *testing.B) {
			b.Run("concmap", func(b *testing.B) {
				tab := fillConc(keys, bbits)
				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					var ctr atomic.Uint64
					for pb.Next() {
						i := ctr.Add(1) - 1
						_, _ = tab.Get(keys[int(i)%sz])
					}
				})
			})
			b.Run("plain", func(b *testing.B) {
				tab := fillPlain(keys)
				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					var ctr atomic.Uint64
					for pb.Next() {
						i := ctr.Add(1) - 1
						_, _ = tab.Get(keys[int(i)%sz])
					}
				})
			})
		})
	}
}

func BenchmarkParallelPutOverwrite(b *testing.B) {
	sizes := envSizes(b)
	bbits := envBucketBits(b)

	for _, sz := range sizes {
		keys := makeKeys(sz)
		b.Run(fmt.Sprintf("size_%d", sz), func(b *testing.B) {
			b.Run("concmap", func(b *testing.B) {
				tab := fillConc(keys, bbits)
				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					var ctr atomic.Uint64
					for pb.Next() {
						i := ctr.Add(1) - 1
						tab.Put(keys[int(i)%sz], int(i))
					}
				})
			})
			b.Run("plain", func(b *testing.B) {
				tab := fillPlain(keys)
				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					var ctr atomic.Uint64
					for pb.Next() {
						i := ctr.Add(1) - 1
						tab.Put(keys[int(i)%sz], int(i))
					}
				})
			})
		})
	}
}

func BenchmarkParallelMixedRW(b *testing.B) {
	sizes := envSizes(b)
	bbits := envBucketBits(b)

	for _, sz := range sizes {
		keys := makeKeys(sz)
		mergeKeys := make([]string, 7)
		for i := range mergeKeys {
			mergeKeys[i] = fmt.Sprintf("mix_%d", i)
		}

		b.Run(fmt.Sprintf("size_%d", sz), func(b *testing.B) {
			b.Run("concmap", func(b *testing.B) {
				tab := fillConc(keys, bbits)
				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					var ctr atomic.Uint64
					for pb.Next() {
						i := ctr.Add(1) - 1
						switch i % 3 {
						case 0:
							_, _ = tab.Get(keys[int(i)%sz])
						case 1:
							tab.Put(keys[int(i)%sz], int(i))
						default:
							_ = tab.Merge(mergeKeys[int(i)%len(mergeKeys)], 1, func(a, b int) int { return a + b })
						}
					}
				})
			})
			b.Run("plain", func(b *testing.B) {
				tab := fillPlain(keys)
				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					var ctr atomic.Uint64
					for pb.Next() {
						i := ctr.Add(1) - 1
						switch i % 3 {
						case 0:
							_, _ = tab.Get(keys[int(i)%sz])
						case 1:
							tab.Put(keys[int(i)%sz], int(i))
						default:
							_ = tab.Merge(mergeKeys[int(i)%len(mergeKeys)], 1, func(a, b int) int { return a + b })
						}
					}
				})
			})
		})
	}
}

func BenchmarkSequentialGetHit(b *testing.B) {
	sizes := envSizes(b)
	bbits := envBucketBits(b)

	for _, sz := range sizes {
		keys := makeKeys(sz)
		b.Run(fmt.Sprintf("size_%d", sz), func(b *testing.B) {
			b.Run("concmap", func(b *testing.B) {
				tab := fillConc(keys, bbits)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, _ = tab.Get(keys[i%sz])
				}
			})
			b.Run("plain", func(b *testing.B) {
				tab := fillPlain(keys)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, _ = tab.Get(keys[i%sz])
				}
			})
			b.Run("unsafe", func(b *testing.B) {
				tab := fillUnsafe(keys)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, _ = tab.Get(keys[i%sz])
				}
			})
		})
	}
}

func fillUnsafe(keys []string) *Unsafe[string, int] {
	m := NewUnsafe[string, int]()
	for i, k := range keys {
		m.Put(k, i)
	}
	return m
}

func BenchmarkRangeFullTable(b *testing.B) {
	sizes := envSizes(b)
	bbits := envBucketBits(b)

	for _, sz := range sizes {
		keys := makeKeys(sz)
		b.Run(fmt.Sprintf("size_%d", sz), func(b *testing.B) {
			b.Run("concmap", func(b *testing.B) {
				tab := fillConc(keys, bbits)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					tab.Range(func(string, int) bool { return true })
				}
			})
			b.Run("plain", func(b *testing.B) {
				tab := fillPlain(keys)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					tab.Range(func(string, int) bool { return true })
				}
			})
		})
	}
}
