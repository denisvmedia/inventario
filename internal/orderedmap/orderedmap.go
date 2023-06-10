package orderedmap

import (
	"container/list"
	"sync"
)

type OrderedMap[T any] struct {
	keys   map[string]*list.Element
	values *list.List
	mutex  sync.RWMutex
}

type Entry[T any] struct {
	Key   string
	Value T
}

func New[T any]() *OrderedMap[T] {
	return &OrderedMap[T]{
		keys:   make(map[string]*list.Element),
		values: list.New(),
	}
}

func (om *OrderedMap[T]) Set(key string, value T) {
	om.mutex.Lock()
	defer om.mutex.Unlock()

	if elem, ok := om.keys[key]; ok {
		// Key already exists, update the value
		elem.Value.(*Entry[T]).Value = value
	} else {
		// Key does not exist, add a new entry
		entry := &Entry[T]{Key: key, Value: value}
		elem := om.values.PushBack(entry)
		om.keys[key] = elem
	}
}

func (om *OrderedMap[T]) Get(key string) (T, bool) {
	om.mutex.RLock()
	defer om.mutex.RUnlock()

	if elem, ok := om.keys[key]; ok {
		entry := elem.Value.(*Entry[T])
		return entry.Value, true
	}
	var zero T
	return zero, false
}

func (om *OrderedMap[T]) Delete(key string) {
	om.mutex.Lock()
	defer om.mutex.Unlock()

	if elem, ok := om.keys[key]; ok {
		om.values.Remove(elem)
		delete(om.keys, key)
	}
}

func (om *OrderedMap[T]) Len() int {
	om.mutex.RLock()
	defer om.mutex.RUnlock()

	return len(om.keys)
}

func (om *OrderedMap[T]) Iterate() []*Entry[T] {
	om.mutex.RLock()
	defer om.mutex.RUnlock()

	entries := make([]*Entry[T], 0, om.Len())
	for elem := om.values.Front(); elem != nil; elem = elem.Next() {
		entries = append(entries, elem.Value.(*Entry[T]))
	}
	return entries
}
