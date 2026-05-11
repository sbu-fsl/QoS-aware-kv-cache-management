package cache

import (
	"time"

	"github.com/sbu-fsl/qos-aware-restoration/internal/storage"
)

// QoSRestore restores blocks using Ford-Fulkerson max-flow to optimally split work
// between recomputation and tier-based restore.
//
// The QoS budget is calculated dynamically (SLO).
// This minimizes wall-clock time by allowing the flow algorithm to decide the optimal split.
//
// Flow network topology (nodes 0..T+3):
//
//	0 = source
//	1 = compute
//	2..T+1 = tiers
//	T+2 = sink
//
// Edges:
//
//	source → compute  : capacity = floor(compute_speed × budget)
//	compute → sink    : capacity = len(blockIDs)
//	source → tier_i   : capacity = floor(tier_speed × budget)
//	tier_i  → sink    : capacity = blocks tier has from request
//
// After max-flow, blocks are assigned: if flow routes through compute, recompute;
// otherwise, restore from tier with available flow capacity.
func (e *Engine) QoSRestore(blockIDs []int) ([]RestoreResult, time.Duration) {
	computeSpeed := e.cfg.InferenceEngine.ComputeSpeed
	computeTimePerBlock := time.Duration(float64(time.Second) / computeSpeed)

	results := make([]RestoreResult, len(blockIDs))

	// --- phase 1: classify blocks ---
	// tierHits[i] = tiers that have blockIDs[i], or empty if not cached
	var (
		tierHits  = make([][]*storage.Tier, len(blockIDs))
		notCached = make([]int, 0)
		cached    = make([]int, 0)
	)

	for i, id := range blockIDs {
		for _, tier := range e.tiers {
			if tier.Has(id) {
				tierHits[i] = append(tierHits[i], tier)
			}
		}

		if len(tierHits[i]) == 0 {
			// not cached anywhere, must compute
			results[i] = RestoreResult{
				BlockID:         id,
				Computed:        true,
				ForceRecomputed: false,
				RestoreTime:     computeTimePerBlock,
			}

			notCached = append(notCached, i)
		} else {
			cached = append(cached, i)
		}
	}

	// all blocks uncached; compute them all
	if len(cached) == 0 {
		return e.finalise(results)
	}

	// All cached blocks are candidates for the flow network — the max-flow algorithm
	// decides the optimal split between recompute and tier restore based on throughput.
	// Per-block heuristics (e.g. "this tier is slower than compute") are wrong here
	// because tier restore is parallel per lane: a slow tier serving many blocks in
	// parallel can still beat sequential recompute for the same blocks.
	recomputeCap := e.cfg.CacheEngine.MaxRecomputeBlocks // -1 = unlimited
	recomputeUsed := 0

	flowCached := cached

	// count how many flowCached blocks each tier actually has
	tierSliceIndex := make(map[string]int, len(e.tiers))
	for i, t := range e.tiers {
		tierSliceIndex[t.Name] = i
	}
	tierSupplyCount := make([]int, len(e.tiers))
	for _, idx := range flowCached {
		seen := make(map[int]bool)
		for _, t := range tierHits[idx] {
			ti := tierSliceIndex[t.Name]
			if !seen[ti] {
				seen[ti] = true
				tierSupplyCount[ti]++
			}
		}
	}

	// Compute the optimal budget T* iteratively.
	// A tier is "saturated" if speed×T* exceeds its actual block supply — it can only
	// contribute its supply, not its full throughput. Remove saturated tiers from the
	// active set, subtract their fixed contribution, and recompute T* until stable.
	// This gives the true minimum makespan when some tiers have limited block counts.
	remaining := float64(len(flowCached))
	activeThroughput := computeSpeed
	for _, tier := range e.tiers {
		activeThroughput += tier.Speed
	}

	saturated := make([]bool, len(e.tiers))
	for {
		budget := remaining / activeThroughput
		changed := false
		for i, t := range e.tiers {
			if saturated[i] {
				continue
			}
			if t.Speed*budget > float64(tierSupplyCount[i]) {
				// this tier runs out before T* — fix its contribution and remove it
				saturated[i] = true
				remaining -= float64(tierSupplyCount[i])
				activeThroughput -= t.Speed
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	budget := remaining / activeThroughput

	// --- phase 2: build flow network with compute as a node ---
	//
	// Nodes:
	//   0       = source
	//   1       = compute
	//   2..T+1  = one per tier (T = len(e.tiers))
	//   T+2     = sink
	computeNode := 1
	T := len(e.tiers)
	sink := T + 2
	total := T + 3

	cap := make([][]int, total)
	for i := range cap {
		cap[i] = make([]int, total)
	}

	// build tier index map
	tierIndex := make(map[string]int, T)
	for i, t := range e.tiers {
		tierIndex[t.Name] = 2 + i // nodes 2..T+1
	}

	// source → compute: capacity = blocks we can compute in budget time
	cap[0][computeNode] = int(computeSpeed * budget)

	// compute → sink: capacity = all flow-eligible cached blocks can be recomputed
	cap[computeNode][sink] = len(flowCached)

	// count cached blocks per tier for source → tier edges
	tierSupply := make(map[int]int, T)
	for _, i := range flowCached {
		seen := make(map[int]bool)
		for _, t := range tierHits[i] {
			node := tierIndex[t.Name]
			if !seen[node] {
				seen[node] = true
				tierSupply[node]++
			}
		}
	}

	// source → tier_i: capacity = blocks we can restore from tier in budget time
	for i, t := range e.tiers {
		tierNode := 2 + i
		cap[0][tierNode] = int(t.Speed * budget)
	}

	// tier_i → sink: capacity = cached blocks available in that tier
	for tierNode, supply := range tierSupply {
		cap[tierNode][sink] = supply
	}

	// --- phase 3: Ford-Fulkerson (Edmonds-Karp) ---
	// Keep original capacities to compute flow after residual mutation.
	origSourceCompute := cap[0][computeNode]
	origSourceTierCap := make(map[int]int, T)
	for i := range e.tiers {
		tierNode := 2 + i
		origSourceTierCap[tierNode] = cap[0][tierNode]
	}

	edmondsKarp(cap, 0, sink, total) // mutates cap into residual

	// flow through node = original - residual
	computeFlow := origSourceCompute - cap[0][computeNode]
	tierFlowAlloc := make(map[int]int, T)
	for i := range e.tiers {
		tierNode := 2 + i
		tierFlowAlloc[tierNode] = origSourceTierCap[tierNode] - cap[0][tierNode]
	}

	// --- phase 4: assign cached blocks ---
	// Separate into "should compute" and "should restore" based on flow.
	type blockSlot struct {
		index int
		id    int
		tiers []*storage.Tier
	}

	// Energy-efficiency cap: clamp flow-assigned recomputes to the remaining budget
	// after phase 1b already consumed some of it.
	if recomputeCap > -1 {
		remaining := max(recomputeCap-recomputeUsed, 0)
		if computeFlow > remaining {
			computeFlow = remaining
		}
	}

	shouldCompute := computeFlow
	toComputeBlocks := make([]blockSlot, 0)
	toRestoreBlocks := make([]blockSlot, 0)

	// miss blocks should be recompute
	for _, idx := range notCached {
		toComputeBlocks = append(toComputeBlocks, blockSlot{idx, blockIDs[idx], nil})
	}

	// from flow-eligible cached items, some might need to recompute
	for _, idx := range flowCached {
		if shouldCompute > 0 {
			toComputeBlocks = append(toComputeBlocks, blockSlot{idx, blockIDs[idx], tierHits[idx]})
			shouldCompute--
		} else {
			toRestoreBlocks = append(toRestoreBlocks, blockSlot{idx, blockIDs[idx], tierHits[idx]})
		}
	}

	// assign computed blocks
	for _, b := range toComputeBlocks {
		// if a block has tiers, it means it was forced to recompute
		tmpRestore := RestoreResult{
			BlockID:         b.id,
			Computed:        true,
			ForceRecomputed: false,
			RestoreTime:     computeTimePerBlock,
		}

		if b.tiers != nil {
			tmpRestore.ForceRecomputed = true
		}

		results[b.index] = tmpRestore
	}

	// assign restored blocks using tier flow allocation
	type tierSlot struct {
		tier      *storage.Tier
		node      int
		remaining int
	}

	slots := make([]tierSlot, 0, T)
	for i, t := range e.tiers {
		tierNode := 2 + i
		alloc := tierFlowAlloc[tierNode]
		if alloc > 0 {
			slots = append(slots, tierSlot{t, tierNode, alloc})
		}
	}

	// sort by speed descending so fastest gets filled first
	for i := 1; i < len(slots); i++ {
		for j := i; j > 0 && slots[j].tier.Speed > slots[j-1].tier.Speed; j-- {
			slots[j], slots[j-1] = slots[j-1], slots[j]
		}
	}

	// greedily assign restore blocks to tier slots
	unassigned := make([]blockSlot, 0)
	for _, b := range toRestoreBlocks {
		assigned := false

		for si := range slots {
			if slots[si].remaining <= 0 {
				continue
			}

			// check that this slot's tier has the block
			hasTier := false
			for _, t := range b.tiers {
				if t.Name == slots[si].tier.Name {
					hasTier = true
					break
				}
			}
			if !hasTier {
				continue
			}

			dur := time.Duration(float64(time.Second) / slots[si].tier.Speed)

			results[b.index] = RestoreResult{
				BlockID:     b.id,
				TierName:    slots[si].tier.Name,
				Computed:    false,
				RestoreTime: dur,
			}

			slots[si].tier.Touch(b.id)
			slots[si].remaining--
			assigned = true

			break
		}
		if !assigned {
			unassigned = append(unassigned, b)
		}
	}

	// blocks with no tier slot available fall back to fastest tier (safety net)
	for _, b := range unassigned {
		best := b.tiers[0]

		for _, t := range b.tiers[1:] {
			if t.Speed > best.Speed {
				best = t
			}
		}

		dur := time.Duration(float64(time.Second) / best.Speed)
		results[b.index] = RestoreResult{
			BlockID:     b.id,
			TierName:    best.Name,
			Computed:    false,
			RestoreTime: dur,
		}

		best.Touch(b.id)
	}

	return e.finalise(results)
}

// finalise auto-stores computed blocks and calculates the overall wall-clock duration.
func (e *Engine) finalise(results []RestoreResult) ([]RestoreResult, time.Duration) {
	// in qos restore, we only store blocks that were computed and not forced to recompute (i.e. they had a tier but flow assigned them to compute)
	var toCompute []int
	for _, r := range results {
		if r.Computed && !r.ForceRecomputed {
			toCompute = append(toCompute, r.BlockID)
		}
	}
	if len(toCompute) > 0 {
		_ = e.Store(toCompute)
	}

	// overall = max(per-tier cumulative restore, sequential compute time)
	tierDur := make(map[string]time.Duration)
	var totalComputeDur time.Duration

	for _, r := range results {
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

	return results, overall
}

// edmondsKarp runs Edmonds-Karp (BFS Ford-Fulkerson) on the capacity matrix cap
// (mutated in-place into the residual graph). Returns total max-flow.
func edmondsKarp(cap [][]int, source, sink, n int) int {
	totalFlow := 0
	for {
		// BFS to find augmenting path
		parent := make([]int, n)
		for i := range parent {
			parent[i] = -1
		}

		parent[source] = source
		queue := []int{source}

		for len(queue) > 0 && parent[sink] == -1 {
			u := queue[0]
			queue = queue[1:]
			for v := range n {
				if parent[v] == -1 && cap[u][v] > 0 {
					parent[v] = u
					queue = append(queue, v)
				}
			}
		}

		if parent[sink] == -1 {
			break // no augmenting path
		}

		// find bottleneck
		pathFlow := int(^uint(0) >> 1)
		for v := sink; v != source; v = parent[v] {
			u := parent[v]
			if cap[u][v] < pathFlow {
				pathFlow = cap[u][v]
			}
		}

		// update residual capacities
		for v := sink; v != source; v = parent[v] {
			u := parent[v]
			cap[u][v] -= pathFlow
			cap[v][u] += pathFlow
		}

		totalFlow += pathFlow
	}

	return totalFlow
}
