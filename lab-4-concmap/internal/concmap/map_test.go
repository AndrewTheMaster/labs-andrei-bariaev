package concmap

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
)

// stringIntMap — общий контракт для параметрических тестов (аналог Searcher в lab-2).
type stringIntMap interface {
	Put(string, int)
	Get(string) (int, bool)
	Merge(string, int, func(int, int) int) int
	Clear()
	Size() uint64
	Range(func(string, int) bool)
}

func allMapImpls() []struct {
	name string
	new  func() stringIntMap
} {
	return []struct {
		name string
		new  func() stringIntMap
	}{
		{"concmap", func() stringIntMap { return New[string, int](8) }},
		{"plain", func() stringIntMap { return NewPlain[string, int]() }},
	}
}

func TestPutGetMergeSize(t *testing.T) {
	m := New[string, int](6)
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("k_%d", i)
		m.Put(key, i)
		v, ok := m.Get(key)
		if !ok || v != i {
			t.Fatalf("Get после Put: got %v %v wanted %v", v, ok, i)
		}
	}
	if m.Size() != 20 {
		t.Fatalf("Size want 20 got %d", m.Size())
	}

	v := m.Merge("k_0", 7, func(a, b int) int { return a + b })
	if v != 7 {
		t.Fatalf("merge existing want 7 got %v", v)
	}
	v2 := m.Merge("new", 3, func(a, b int) int { return a + b })
	if v2 != 3 {
		t.Fatalf("merge absent want 3 got %v", v2)
	}
	if m.Size() != 21 {
		t.Fatalf("Size after merge inserts want 21 got %d", m.Size())
	}

	cnt := 0
	m.Range(func(k string, v int) bool {
		cnt++
		return true
	})
	if cnt != 21 {
		t.Fatalf("Range count got %d", cnt)
	}

	m.Clear()
	if m.Size() != 0 {
		t.Fatalf("Clear size wanted 0 got %d", m.Size())
	}
	_, ok := m.Get("k_10")
	if ok {
		t.Fatalf("expected miss after clear")
	}
}

func TestPutOverwriteNoSizeIncrease(t *testing.T) {
	m := New[string, int](4)
	m.Put("a", 1)
	m.Put("a", 2)
	if m.Size() != 1 {
		t.Fatalf("size want 1 got %d", m.Size())
	}
}

func TestHappensBeforePutGet(t *testing.T) {
	m := New[string, int](4)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.Put("x", 42)
	}()
	wg.Wait()
	v, ok := m.Get("x")
	if !ok || v != 42 {
		t.Fatalf("happens-before read got %v %v", v, ok)
	}
}

// TestMapMatchesUnsafeOracle — последовательная сверка с не-thread-safe map (оракул в стиле lab-2).
func TestMapMatchesUnsafeOracle(t *testing.T) {
	seeds := []uint64{1, 7, 42, 99, 2026}
	const ops = 5000

	for _, seed := range seeds {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			ref := NewUnsafe[string, int]()
			m := New[string, int](10)
			r := rand.NewPCG(seed, seed^0xdeadbeef)

			for i := 0; i < ops; i++ {
				key := fmt.Sprintf("k_%d", r.Uint64()%128)
				switch r.Uint64() % 5 {
				case 0:
					v := int(r.Uint64() % 1000)
					ref.Put(key, v)
					m.Put(key, v)
				case 1:
					want, wok := ref.Get(key)
					got, gok := m.Get(key)
					if wok != gok || (wok && got != want) {
						t.Fatalf("Get drift key=%q ref=%v,%v got=%v,%v", key, want, wok, got, gok)
					}
				case 2:
					delta := int(r.Uint64()%10) + 1
					want := ref.Merge(key, delta, func(a, b int) int { return a + b })
					got := m.Merge(key, delta, func(a, b int) int { return a + b })
					if got != want {
						t.Fatalf("Merge drift key=%q want=%v got=%v", key, want, got)
					}
				case 3:
					if r.Uint64()%50 == 0 {
						ref.Clear()
						m.Clear()
					}
				default:
					if ref.Size() != m.Size() {
						t.Fatalf("Size drift want %d got %d", ref.Size(), m.Size())
					}
				}
			}

			ref.Range(func(k string, v int) bool {
				got, ok := m.Get(k)
				if !ok || got != v {
					t.Fatalf("final Get key=%q ref=%v got=%v ok=%v", k, v, got, ok)
				}
				return true
			})
		})
	}
}

// TestMapImplsMatchUnsafeOracle — concmap и plain совпадают с оракулом в однопоточном режиме.
func TestMapImplsMatchUnsafeOracle(t *testing.T) {
	seeds := []uint64{3, 17}
	const ops = 2000

	for _, tc := range allMapImpls() {
		tc := tc
		for _, seed := range seeds {
			t.Run(fmt.Sprintf("%s/seed=%d", tc.name, seed), func(t *testing.T) {
				ref := NewUnsafe[string, int]()
				m := tc.new()
				r := rand.NewPCG(seed, 1)

				for i := 0; i < ops; i++ {
					key := fmt.Sprintf("x_%d", r.Uint64()%64)
					v := int(r.Uint64() % 500)
					ref.Put(key, v)
					m.Put(key, v)
					want, wok := ref.Get(key)
					got, gok := m.Get(key)
					if wok != gok || got != want {
						t.Fatalf("Get %s key=%q", tc.name, key)
					}
				}
				if ref.Size() != m.Size() {
					t.Fatalf("%s Size want %d got %d", tc.name, ref.Size(), m.Size())
				}
			})
		}
	}
}

func TestSizeZeroAfterClearConcurrent(t *testing.T) {
	m := New[string, int](6)
	for i := 0; i < 100; i++ {
		m.Put(fmt.Sprintf("k_%d", i), i)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		m.Clear()
	}()
	go func() {
		defer wg.Done()
		for j := 0; j < 50; j++ {
			_ = m.Size()
			_, _ = m.Get("k_0")
		}
	}()
	wg.Wait()
	if m.Size() != 0 {
		t.Fatalf("after concurrent clear Size want 0 got %d", m.Size())
	}
}
