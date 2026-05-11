package cache

import "time"

// BlockLocation records which tier a block was found in during restore.
type BlockLocation struct {
	BlockID  int
	TierName string
}

// LookupResult is returned per block for a lookup.
type LookupResult struct {
	BlockID   int
	Locations []string // tier names that have this block
	Hit       bool
}

// RestoreResult describes what happened when restoring a single block.
type RestoreResult struct {
	BlockID         int
	TierName        string        // tier it was found in, or "" if computed
	Computed        bool          // true if not found anywhere, had to compute
	ForceRecomputed bool          // true if recompute was faster than any available tier (QoS mode only)
	RestoreTime     time.Duration // simulated time for this block from its tier
}

// StoreResult describes what happened when storing a single block.
type StoreResult struct {
	BlockID int
	Evicted map[string]int // tier -> evicted block (-1 if none)
}

// TierStatus returns a snapshot of each tier's fill.
type TierStatus struct {
	Name     string
	Used     int
	Capacity int
	Blocks   []int
}
