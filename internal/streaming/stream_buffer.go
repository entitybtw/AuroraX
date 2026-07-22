package streaming

import (
	"sync"
)

const (
	chunkInitSize   = 1024
	chunkMaxCached  = 64 * 1024
)

var chunkPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, chunkInitSize)
		return &buf
	},
}

type ChunkBuffer struct {
	store  []byte
	handle *[]byte
	offset int
}

func NewChunkBuffer(capacity int) ChunkBuffer {
	if capacity <= 0 {
		capacity = chunkInitSize
	}

	pooled := chunkPool.Get().(*[]byte)
	data := (*pooled)[:0]
	if cap(data) == 0 || cap(data) > chunkMaxCached {
		data = make([]byte, 0, chunkInitSize)
		*pooled = data
	}
	if cap(data) < capacity {
		data = make([]byte, 0, capacity)
	}

	return ChunkBuffer{
		store:  data[:0],
		handle: pooled,
	}
}

func (b *ChunkBuffer) Len() int {
	if b == nil || b.offset >= len(b.store) {
		return 0
	}
	return len(b.store) - b.offset
}

func (b *ChunkBuffer) Unread() []byte {
	if b.Len() == 0 {
		return nil
	}
	return b.store[b.offset:]
}

func (b *ChunkBuffer) AppendBytes(data []byte) {
	if len(data) == 0 {
		return
	}
	b.prepareAppend()
	b.store = append(b.store, data...)
}

func (b *ChunkBuffer) AppendString(data string) {
	if data == "" {
		return
	}
	b.prepareAppend()
	b.store = append(b.store, data...)
}

func (b *ChunkBuffer) Read(p []byte) int {
	if len(p) == 0 || b.Len() == 0 {
		return 0
	}
	n := copy(p, b.store[b.offset:])
	b.Discard(n)
	return n
}

func (b *ChunkBuffer) Discard(n int) {
	if n <= 0 || b.Len() == 0 {
		return
	}
	if n >= b.Len() {
		b.store = b.store[:0]
		b.offset = 0
		return
	}
	b.offset += n
}

func (b *ChunkBuffer) Release() {
	if b == nil {
		return
	}
	if b.handle != nil {
		pooled := (*b.handle)[:0]
		if cap(pooled) == 0 || cap(pooled) > chunkMaxCached {
			pooled = make([]byte, 0, chunkInitSize)
		}
		*b.handle = pooled
		chunkPool.Put(b.handle)
	}
	b.store = nil
	b.handle = nil
	b.offset = 0
}

func (b *ChunkBuffer) prepareAppend() {
	switch {
	case b.store == nil:
		*b = NewChunkBuffer(chunkInitSize)
	case b.offset == 0:
		return
	case b.offset >= len(b.store):
		b.store = b.store[:0]
		b.offset = 0
	default:
		copy(b.store, b.store[b.offset:])
		b.store = b.store[:len(b.store)-b.offset]
		b.offset = 0
	}
}

type StreamBuffer = ChunkBuffer

func NewStreamBuffer(capacity int) StreamBuffer {
	return NewChunkBuffer(capacity)
}

func (b *StreamBuffer) Consume(n int) {
	b.Discard(n)
}
