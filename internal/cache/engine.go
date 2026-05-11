package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/sbu-fsl/qos-aware-restoration/internal/config"
	"github.com/sbu-fsl/qos-aware-restoration/internal/storage"
)

// Engine manages blocks across multiple tiers.
type Engine struct {
	mu    sync.RWMutex
	cfg   *config.Config
	tiers []*storage.Tier
}

// NewEngine initializes the cache engine with the given config and data directory for tier storage.
func NewEngine(cfg *config.Config, dataDir string) (*Engine, error) {
	var (
		e                = &Engine{cfg: cfg}
		hasVRAM, hasDRAM bool
	)

	for _, tc := range cfg.CacheEngine.Tiers {
		if tc.Name == "VRAM" {
			hasVRAM = true
		}
		if tc.Name == "DRAM" {
			hasDRAM = true
		}

		// each tier gets its own file-based storage with eviction callback to the engine
		tier, err := storage.NewTier(tc.Name, tc.Capacity, tc.RestoreSpeed, dataDir)
		if err != nil {
			return nil, fmt.Errorf("init tier %s: %w", tc.Name, err)
		}

		e.tiers = append(e.tiers, tier)
	}

	// ensure VRAM and DRAM are present
	if !hasVRAM {
		return nil, fmt.Errorf("config must include a VRAM tier")
	}
	if !hasDRAM {
		return nil, fmt.Errorf("config must include a DRAM tier")
	}

	return e, nil
}

// Store stores blockIDs. First stores in VRAM and DRAM.
// If DRAM evicts something because of this store, the evicted block is then
// stored in all other tiers (except VRAM and DRAM) to keep it cached.
func (e *Engine) Store(blockIDs []int) []StoreResult {
	results := make([]StoreResult, len(blockIDs))

	for i, id := range blockIDs {
		res := StoreResult{BlockID: id, Evicted: make(map[string]int)}

		// store in VRAM and DRAM first
		var evictedFromDRAM int
		for _, tier := range e.tiers {
			if tier.Name == "VRAM" {
				_, _ = tier.Store(id) // VRAM eviction doesn't cascade, so store but don't track evictions
				continue
			}
			if tier.Name == "DRAM" {
				evicted, _ := tier.Store(id)
				evictedFromDRAM = evicted
			}
		}

		// if DRAM evicted something, store in all other tiers except VRAM and DRAM
		if evictedFromDRAM > 0 {
			for _, tier := range e.tiers {
				if tier.Name != "VRAM" && tier.Name != "DRAM" {
					_, _ = tier.Store(evictedFromDRAM)
				}
			}
		}

		results[i] = res
	}

	return results
}

// Lookup checks which tiers have each block.
func (e *Engine) Lookup(blockIDs []int) []LookupResult {
	results := make([]LookupResult, len(blockIDs))

	for i, id := range blockIDs {
		res := LookupResult{BlockID: id}

		for _, tier := range e.tiers {
			if tier.Has(id) {
				res.Locations = append(res.Locations, tier.Name)
				res.Hit = true
			}
		}

		results[i] = res
	}

	return results
}

// RestoreAuto dispatches to QoSRestore or Restore based on the configured restore_mode.
func (e *Engine) RestoreAuto(blockIDs []int) ([]RestoreResult, time.Duration) {
	if e.cfg.CacheEngine.RestoreMode == "qos" {
		return e.QoSRestore(blockIDs)
	}

	return e.Restore(blockIDs)
}

// Restore restores blocks. Each block is fetched from the fastest tier that has it.
// Missing blocks are "computed" and then auto-stored. Returns per-block results and
// the overall wall-clock time (parallel restore across tiers).
func (e *Engine) Restore(blockIDs []int) ([]RestoreResult, time.Duration) {
	type work struct {
		index int
		id    int
	}

	// group blocks by which tier serves them (prefer first/fastest tier).
	tierAssignment := make([]RestoreResult, len(blockIDs))
	var toCompute []int

	for i, id := range blockIDs {
		found := false

		for _, tier := range e.tiers {
			if tier.Has(id) {
				blocksPerSec := tier.Speed

				// calculate QoS
				dur := time.Duration(float64(time.Second) / blocksPerSec)

				tierAssignment[i] = RestoreResult{
					BlockID:     id,
					TierName:    tier.Name,
					Computed:    false,
					RestoreTime: dur,
				}

				tier.Touch(id)
				found = true

				break
			}
		}

		// must compute (block not found in any tier)
		if !found {
			blocksPerSec := e.cfg.InferenceEngine.ComputeSpeed

			// calculate QoS
			dur := time.Duration(float64(time.Second) / blocksPerSec)

			tierAssignment[i] = RestoreResult{
				BlockID:     id,
				TierName:    "",
				Computed:    true,
				RestoreTime: dur,
			}

			toCompute = append(toCompute, id)
		}
	}

	// auto-store computed blocks for next time
	if len(toCompute) > 0 {
		_ = e.Store(toCompute)
	}

	// overall time = max(per-tier cumulative restore) , sum(sequential compute times)
	var (
		tierDur         = make(map[string]time.Duration)
		totalComputeDur time.Duration
	)
	for _, r := range tierAssignment {
		if r.Computed {
			totalComputeDur += r.RestoreTime
		} else {
			tierDur[r.TierName] += r.RestoreTime
		}
	}

	var maxRestoreDur time.Duration
	for _, d := range tierDur {
		if d > maxRestoreDur {
			maxRestoreDur = d
		}
	}

	overall := max(maxRestoreDur, totalComputeDur)

	return tierAssignment, overall
}

// Status returns a snapshot of each tier's fill.
func (e *Engine) Status() []TierStatus {
	out := make([]TierStatus, len(e.tiers))

	for i, tier := range e.tiers {
		out[i] = TierStatus{
			Name:     tier.Name,
			Used:     tier.Count(),
			Capacity: tier.Capacity,
			Blocks:   tier.BlockIDs(),
		}
	}

	return out
}
