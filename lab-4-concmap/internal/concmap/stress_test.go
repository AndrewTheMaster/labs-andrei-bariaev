package concmap

import (
	"math/rand/v2"
	"strconv"
	"sync"
	"testing"
)

// Стресс и гонки: make test-race (go test -race -count=3), аналог jcstress для Go.
func TestStressMergeAdditiveRace(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	const goroutines = 64
	const itersPerG = 2000

	m := New[string, int](10)
	sumSerial := map[string]int{}
	var mux sync.Mutex

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(id int) {
			defer wg.Done()
			src := rand.NewPCG(uint64(id), 777)
			for i := 0; i < itersPerG; i++ {
				k := strconv.Itoa(int(src.Uint64() % 32))
				delta := int(src.Uint64()%5) + 1
				got := m.Merge(k, delta, func(a, b int) int { return a + b })
				_ = got
				mux.Lock()
				sumSerial[k] += delta
				mux.Unlock()
				if i%17 == 0 {
					_, _ = m.Get(k)
				}
			}
		}(g)
	}
	wg.Wait()

	for k, want := range sumSerial {
		got, ok := m.Get(k)
		if !ok || got != want {
			t.Fatalf("merge drift key=%q got=%v ok=%v want=%v", k, got, ok, want)
		}
	}
}

func TestStressPlainVsConcNoPanicRace(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	run := func(name string, work func()) {
		t.Run(name, func(t *testing.T) {
			var wg sync.WaitGroup
			for i := 0; i < 32; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					work()
				}()
			}
			wg.Wait()
		})
	}

	run("conc", func() {
		m := New[int, int](8)
		for i := 0; i < 500; i++ {
			m.Put(i%50, i)
			_, _ = m.Get(i % 50)
			_ = m.Merge(i%7, 1, func(a, b int) int { return a + b })
			if i%97 == 0 {
				m.Range(func(k int, v int) bool { return true })
			}
			if i%401 == 0 {
				m.Clear()
			}
		}
	})

	run("plain", func() {
		p := NewPlain[int, int]()
		for i := 0; i < 500; i++ {
			p.Put(i%50, i)
			_, _ = p.Get(i % 50)
			_ = p.Merge(i%7, 1, func(a, b int) int { return a + b })
			if i%97 == 0 {
				p.Range(func(k int, v int) bool { return true })
			}
			if i%401 == 0 {
				p.Clear()
			}
		}
	})
}
