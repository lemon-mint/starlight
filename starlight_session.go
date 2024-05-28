package starlight

import (
	"sync"
	"sync/atomic"
)

type starlightSessionBucket struct {
	mu       sync.RWMutex
	sessions map[uint64]*starlightSession
}

const buckets_n = 1 << 8
const buckets_mask = buckets_n - 1

type starlightSessionPool struct {
	id_counter uint64
	buckets    [buckets_n]starlightSessionBucket
}

func (g *starlightSessionPool) getID() uint64 {
	return atomic.AddUint64(&g.id_counter, 1)
}

func (g *starlightSessionPool) GetSession(id uint64) (*starlightSession, bool) {
	idx := id & buckets_mask

	if g.buckets[idx].sessions == nil {
		return nil, false
	}

	g.buckets[idx].mu.RLock()
	defer g.buckets[idx].mu.RUnlock()

	sess, ok := g.buckets[idx].sessions[id]
	return sess, ok
}

type starlightSession struct {
	ID uint64
}
