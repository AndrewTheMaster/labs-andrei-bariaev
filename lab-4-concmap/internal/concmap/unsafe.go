package concmap

// Unsafe[K,V] — встроенная map без синхронизации.
// Используется только в однопоточных тестах/бенчмарках как эталон «сырой» map (не thread-safe).
type Unsafe[K comparable, V any] struct {
	m map[K]V
}

func NewUnsafe[K comparable, V any]() *Unsafe[K, V] {
	return &Unsafe[K, V]{m: make(map[K]V)}
}

func (u *Unsafe[K, V]) Put(key K, val V) {
	u.m[key] = val
}

func (u *Unsafe[K, V]) Get(key K) (V, bool) {
	v, ok := u.m[key]
	return v, ok
}

func (u *Unsafe[K, V]) Merge(key K, val V, merger func(existing, incoming V) V) V {
	if merger == nil {
		panic("unsafe: Merge nil merger")
	}
	if old, ok := u.m[key]; ok {
		nv := merger(old, val)
		u.m[key] = nv
		return nv
	}
	u.m[key] = val
	return val
}

func (u *Unsafe[K, V]) Clear() {
	clear(u.m)
}

func (u *Unsafe[K, V]) Size() uint64 {
	return uint64(len(u.m))
}

func (u *Unsafe[K, V]) Range(fn func(K, V) bool) {
	for k, v := range u.m {
		if !fn(k, v) {
			return
		}
	}
}
