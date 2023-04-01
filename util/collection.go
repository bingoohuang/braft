package util

import "sync"

type UniqueQueue struct {
	elems        []string
	maxQueueSize int
	sync.Mutex
}

func NewUniqueQueue(maxQueueSize int) *UniqueQueue {
	return &UniqueQueue{
		maxQueueSize: maxQueueSize,
		elems:        make([]string, 0, maxQueueSize+1),
	}
}

func (q *UniqueQueue) Get() []string {
	q.Lock()
	defer q.Unlock()

	ret := make([]string, len(q.elems))
	copy(ret, q.elems)
	return ret
}

func (q *UniqueQueue) Put(value string) {
	q.Lock()
	defer q.Unlock()

	for i, elem := range q.elems {
		if elem == value {
			copy(q.elems[i:], q.elems[i+1:])
			q.elems[len(q.elems)-1] = value
			return
		}
	}

	q.elems = append(q.elems, value)

	if len(q.elems) > q.maxQueueSize {
		copy(q.elems, q.elems[1:])
		q.elems = q.elems[:q.maxQueueSize]
	}
}
