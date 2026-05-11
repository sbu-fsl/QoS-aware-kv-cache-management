package storage

// LRUEntry represents an entry in the LRU list for a tier.
type LRUEntry struct {
	blockID int
	prev    *LRUEntry
	next    *LRUEntry
}
