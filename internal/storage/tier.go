package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Tier is a single storage tier backed by a JSON file on disk.
type Tier struct {
	mu sync.Mutex

	Name     string
	Capacity int
	Speed    float64 // blocks per second for restore

	inmemoryRun bool // if true, operates in-memory without file persistence (for less overhead in tests)
	filePath    string
	blocks      map[int]struct{} // present block IDs

	// LRU doubly-linked list; head = most recent, tail = least recent
	head    *LRUEntry
	tail    *LRUEntry
	entries map[int]*LRUEntry
}

// tierState is the JSON-serializable state of a tier, containing the LRU order of block IDs.
type tierState struct {
	Order []int `json:"order"` // LRU order, index 0 = most recent
}

// NewTier constructs a new tier with the given parameters and loads its state from disk.
func NewTier(name string, capacity int, speed float64, dataDir string, inmemoryRun bool) (*Tier, error) {
	t := &Tier{
		Name:        name,
		Capacity:    capacity,
		Speed:       speed,
		filePath:    fmt.Sprintf("%s/%s.json", dataDir, name),
		inmemoryRun: inmemoryRun,
		blocks:      make(map[int]struct{}),
		entries:     make(map[int]*LRUEntry),
	}

	if err := t.load(); err != nil {
		return nil, err
	}

	return t, nil
}

// load reads the tier state from its backing file and reconstructs the in-memory structures.
func (t *Tier) load() error {
	if t.inmemoryRun {
		return nil
	}

	data, err := os.ReadFile(t.filePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read data file: %w", err)
	}

	var state tierState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse data file: %w", err)
	}

	// rebuild LRU from saved order (index 0 = most recent).
	for _, id := range state.Order {
		t.blocks[id] = struct{}{}
		e := &LRUEntry{blockID: id}
		t.entries[id] = e
		if t.head == nil {
			t.head = e
			t.tail = e
		} else {
			e.next = t.head
			t.head.prev = e
			t.head = e
		}
	}

	return nil
}

// save writes the current tier state to its backing file.
func (t *Tier) save() error {
	order := make([]int, 0, len(t.blocks))

	for cur := t.head; cur != nil; cur = cur.next {
		order = append(order, cur.blockID)
	}

	if t.inmemoryRun {
		return nil
	}

	data, err := json.MarshalIndent(tierState{Order: order}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to store data: %w", err)
	}

	return os.WriteFile(t.filePath, data, 0644)
}

// Store adds a block. If capacity is exceeded the LRU block is evicted.
// Returns the evicted block ID (-1 if none).
func (t *Tier) Store(blockID int) (evicted int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	evicted = -1

	if _, exists := t.blocks[blockID]; exists {
		t.touch(blockID)
		return evicted, t.save()
	}

	if len(t.blocks) >= t.Capacity {
		evicted = t.evictLRU()
	}

	t.blocks[blockID] = struct{}{}
	e := &LRUEntry{blockID: blockID}
	t.entries[blockID] = e
	t.pushFront(e)

	return evicted, t.save()
}

// Has returns true if the block is present.
func (t *Tier) Has(blockID int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	_, ok := t.blocks[blockID]

	return ok
}

// Touch marks a block as recently used (called on restore hit).
func (t *Tier) Touch(blockID int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.touch(blockID)
	_ = t.save()
}

// touch moves the block to the front of the LRU list. Caller must hold lock.
func (t *Tier) touch(blockID int) {
	e, ok := t.entries[blockID]
	if !ok {
		return
	}

	t.remove(e)
	t.pushFront(e)
}

// evictLRU removes the least recently used block and returns its ID, or -1 if empty. Caller must hold lock.
func (t *Tier) evictLRU() int {
	if t.tail == nil {
		return -1
	}

	id := t.tail.blockID
	t.remove(t.tail)

	delete(t.blocks, id)
	delete(t.entries, id)

	return id
}

// pushFront adds an entry to the front of the LRU list. Caller must hold lock.
func (t *Tier) pushFront(e *LRUEntry) {
	e.prev = nil
	e.next = t.head
	if t.head != nil {
		t.head.prev = e
	}

	t.head = e
	if t.tail == nil {
		t.tail = e
	}
}

// remove removes an entry from the LRU list. Caller must hold lock.
func (t *Tier) remove(e *LRUEntry) {
	if e.prev != nil {
		e.prev.next = e.next
	} else {
		t.head = e.next
	}

	if e.next != nil {
		e.next.prev = e.prev
	} else {
		t.tail = e.prev
	}

	e.prev = nil
	e.next = nil
}

// Remove deletes a specific block from the tier without triggering the eviction callback.
func (t *Tier) Remove(blockID int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.blocks[blockID]; !ok {
		return nil
	}

	e := t.entries[blockID]
	t.remove(e)
	delete(t.blocks, blockID)
	delete(t.entries, blockID)

	return t.save()
}

// Purge removes all blocks from the tier and clears its backing file.
func (t *Tier) Purge() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.blocks = make(map[int]struct{})
	t.entries = make(map[int]*LRUEntry)
	t.head = nil
	t.tail = nil

	if t.inmemoryRun {
		return nil
	}

	return os.WriteFile(t.filePath, []byte(`{"order":[]}`), 0644)
}

// BlockIDs returns all stored block IDs.
func (t *Tier) BlockIDs() []int {
	t.mu.Lock()
	defer t.mu.Unlock()

	ids := make([]int, 0, len(t.blocks))
	for id := range t.blocks {
		ids = append(ids, id)
	}

	return ids
}

// Count returns the number of stored blocks.
func (t *Tier) Count() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return len(t.blocks)
}
