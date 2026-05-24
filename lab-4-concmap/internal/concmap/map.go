// Package concmap — striping hash-table с закрытой адресацией (цепочки в бакетах)
// и отдельной RWMutex на бакет. Чтение Get/Range блокируется только записью в тот же бакет,
// но не блокируется параллельными чтениями и записями в другие бакеты (принцип сегментов CHM JDK).
//
// При росте числа ключей таблица удваивает число бакетов (rehash), если size > loadFactor * buckets.
//
// Наблюдаемый порядок (happens-before) между завершёнными операциями:
// отпускание мьютекса записи перед захватом мьютекса читателя/писателя того же или другого бакета,
// см. память-порядок sync пакета (аналог документации ConcurrentHashMap).
package concmap

import (
	"encoding/binary"
	"hash/maphash"
	"sync"
	"sync/atomic"
)

const defaultLoadFactor = 0.75

// Option параметризация конструктора.
type Option[K comparable, V any] func(*Map[K, V])

// WithHasher задаёт пользовательскую функцию хеширования ключа в uint64 (младшие биты мапятся в бакет).
func WithHasher[K comparable, V any](fn func(K) uint64) Option[K, V] {
	return func(m *Map[K, V]) {
		if fn == nil {
			panic("concmap: WithHasher(nil)")
		}
		m.hash = fn
	}
}

// WithLoadFactor задаёт порог rehash: resize, когда size > loadFactor * len(buckets). По умолчанию 0.75.
func WithLoadFactor[K comparable, V any](lf float64) Option[K, V] {
	return func(m *Map[K, V]) {
		if lf <= 0 || lf > 1 {
			panic("concmap: WithLoadFactor вне (0,1]")
		}
		m.loadFactor = lf
	}
}

// WithMaxBucketBits ограничивает максимум 2^bits бакетов при resize (по умолчанию 26).
func WithMaxBucketBits[K comparable, V any](bits uint8) Option[K, V] {
	return func(m *Map[K, V]) {
		if bits < 1 || bits > 26 {
			panic("concmap: WithMaxBucketBits вне [1..26]")
		}
		m.maxBucketBits = bits
	}
}

type bucket[K comparable, V any] struct {
	mu   sync.RWMutex
	head *node[K, V]
}

type node[K comparable, V any] struct {
	key  K
	val  V
	next *node[K, V]
}

type segmentTable[K comparable, V any] struct {
	buckets []bucket[K, V]
	mask    uint64
}

// Map[K,V] потокобезопасная хеш-таблица.
type Map[K comparable, V any] struct {
	tab           atomic.Pointer[segmentTable[K, V]]
	bucketBits    uint8
	maxBucketBits uint8
	loadFactor    float64
	hash          func(K) uint64
	size          atomic.Uint64
	resizeMu      sync.Mutex
}

// New создаёт таблицу с 2^bucketBits бакетами (bucketBits в [1..26]).
func New[K comparable, V any](bucketBits uint8, opts ...Option[K, V]) *Map[K, V] {
	if bucketBits < 1 || bucketBits > 26 {
		panic("concmap: bucketBits должно быть в [1..26]")
	}
	m := &Map[K, V]{
		bucketBits:    bucketBits,
		maxBucketBits: 26,
		loadFactor:    defaultLoadFactor,
	}
	for _, o := range opts {
		o(m)
	}
	if m.hash == nil {
		m.hash = makeDefaultHashFunc[K]()
	}
	m.tab.Store(newSegmentTable[K, V](bucketBits))
	return m
}

func newSegmentTable[K comparable, V any](bucketBits uint8) *segmentTable[K, V] {
	n := uint64(1) << bucketBits
	return &segmentTable[K, V]{
		buckets: make([]bucket[K, V], n),
		mask:    n - 1,
	}
}

// makeDefaultHashFunc выбирает специализированное хеширование без reflect.ValueOf(K) для hot-path string/int.
func makeDefaultHashFunc[K comparable]() func(K) uint64 {
	var probe K
	switch any(probe).(type) {
	case string:
		seed := maphash.MakeSeed()
		return func(k K) uint64 {
			var h maphash.Hash
			h.SetSeed(seed)
			_, _ = h.WriteString(any(k).(string))
			return h.Sum64()
		}
	case int:
		seed := maphash.MakeSeed()
		return func(k K) uint64 {
			var h maphash.Hash
			h.SetSeed(seed)
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], uint64(any(k).(int)))
			_, _ = h.Write(buf[:])
			return h.Sum64()
		}
	case int64:
		seed := maphash.MakeSeed()
		return func(k K) uint64 {
			var h maphash.Hash
			h.SetSeed(seed)
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], uint64(any(k).(int64)))
			_, _ = h.Write(buf[:])
			return h.Sum64()
		}
	default:
		seed := maphash.MakeSeed()
		return newDefaultHasher[K](seed)
	}
}

func (m *Map[K, V]) table() *segmentTable[K, V] {
	return m.tab.Load()
}

func (m *Map[K, V]) bucketOf(t *segmentTable[K, V], key K) *bucket[K, V] {
	i := int(m.hash(key) & t.mask)
	return &t.buckets[i]
}

func (m *Map[K, V]) resizeThreshold(t *segmentTable[K, V]) uint64 {
	return uint64(float64(len(t.buckets)) * m.loadFactor)
}

func (m *Map[K, V]) needsResize(t *segmentTable[K, V]) bool {
	return m.size.Load() > m.resizeThreshold(t) && m.bucketBits < m.maxBucketBits
}

func (m *Map[K, V]) insertInto(t *segmentTable[K, V], n *node[K, V]) {
	b := m.bucketOf(t, n.key)
	b.mu.Lock()
	n.next = b.head
	b.head = n
	b.mu.Unlock()
}

func (m *Map[K, V]) resize() {
	m.resizeMu.Lock()
	defer m.resizeMu.Unlock()

	old := m.table()
	if !m.needsResize(old) {
		return
	}
	if m.bucketBits >= m.maxBucketBits {
		return
	}

	newBits := m.bucketBits + 1
	newTab := newSegmentTable[K, V](newBits)

	for i := range old.buckets {
		old.buckets[i].mu.Lock()
	}
	if !m.needsResize(old) {
		for i := range old.buckets {
			old.buckets[i].mu.Unlock()
		}
		return
	}

	for i := range old.buckets {
		for cur := old.buckets[i].head; cur != nil; {
			next := cur.next
			cur.next = nil
			m.insertInto(newTab, cur)
			cur = next
		}
		old.buckets[i].head = nil
	}

	m.tab.Store(newTab)
	m.bucketBits = newBits

	for i := range old.buckets {
		old.buckets[i].mu.Unlock()
	}
}

// Put вставляет или перезаписывает ключ. Если ключ новый — size увеличивается; при переполнении — resize.
func (m *Map[K, V]) Put(key K, val V) {
	t := m.table()
	b := m.bucketOf(t, key)
	b.mu.Lock()
	for cur := b.head; cur != nil; cur = cur.next {
		if cur.key == key {
			cur.val = val
			b.mu.Unlock()
			return
		}
	}
	b.head = &node[K, V]{key: key, val: val, next: b.head}
	m.size.Add(1)
	b.mu.Unlock()
	if m.needsResize(t) {
		m.resize()
	}
}

// Get без блокировки чужих бакетов; разделяет RLock только с братскими операциями в том же бакете.
func (m *Map[K, V]) Get(key K) (V, bool) {
	t := m.table()
	b := m.bucketOf(t, key)
	b.mu.RLock()
	for cur := b.head; cur != nil; cur = cur.next {
		if cur.key == key {
			v := cur.val
			b.mu.RUnlock()
			return v, true
		}
	}
	var zero V
	b.mu.RUnlock()
	return zero, false
}

// Merge как в JDK: если ключа не было — сохраняет value без вызова merger и возвращает его;
// иначе newVal := merger(existing, val), сохраняет newVal и возвращает его.
func (m *Map[K, V]) Merge(key K, val V, merger func(existing, incoming V) V) V {
	if merger == nil {
		panic("concmap: Merge(..., merger: nil)")
	}
	t := m.table()
	b := m.bucketOf(t, key)
	b.mu.Lock()
	for cur := b.head; cur != nil; cur = cur.next {
		if cur.key == key {
			nv := merger(cur.val, val)
			cur.val = nv
			b.mu.Unlock()
			return nv
		}
	}
	b.head = &node[K, V]{key: key, val: val, next: b.head}
	m.size.Add(1)
	b.mu.Unlock()
	if m.needsResize(t) {
		m.resize()
	}
	return val
}

// Clear удаляет все пары под глобальным порядком блокировки бакетов (слева направо) против взаимоблокировок.
func (m *Map[K, V]) Clear() {
	m.resizeMu.Lock()
	defer m.resizeMu.Unlock()

	t := m.table()
	for i := range t.buckets {
		t.buckets[i].mu.Lock()
	}
	for i := range t.buckets {
		t.buckets[i].head = nil
	}
	m.size.Store(0)
	for i := range t.buckets {
		t.buckets[i].mu.Unlock()
	}
}

// Size число ключей (~ JDK size() точный при отсутствии гонок на Clear).
func (m *Map[K, V]) Size() uint64 {
	return m.size.Load()
}

// BucketCount число бакетов (для тестов и отладки).
func (m *Map[K, V]) BucketCount() int {
	return len(m.table().buckets)
}

// Range обходит ключи; итерация сопоставима с «слабо согласованным» видом JDK:
// не бросает при конкуррентной модификации, но может не видеть одновременно вставленные элементы других потоков.
// Для каждого бакета чтение идёт под RLock, поэтому структура цепочки в бакете стабильна.
func (m *Map[K, V]) Range(fn func(key K, val V) bool) {
	t := m.table()
	for i := range t.buckets {
		b := &t.buckets[i]
		b.mu.RLock()
		for cur := b.head; cur != nil; cur = cur.next {
			if !fn(cur.key, cur.val) {
				b.mu.RUnlock()
				return
			}
		}
		b.mu.RUnlock()
	}
}
