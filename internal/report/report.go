package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/sbu-fsl/qos-aware-restoration/internal/cache"
)

// Store generates a report for store operations,
// showing which blocks were stored and any evictions that occurred.
func Store(results []cache.StoreResult, verbose bool) string {
	var b strings.Builder

	fmt.Fprintf(&b, "STORE — %d block(s)\n", len(results))
	fmt.Fprintf(&b, "%-10s  %s\n", "Block", "Evictions")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 50))

	if verbose {
		for _, r := range results {
			evictions := []string{}
			for tier, evicted := range r.Evicted {
				if evicted >= 0 {
					evictions = append(evictions, fmt.Sprintf("%s→evicted #%d", tier, evicted))
				}
			}

			ev := "none"
			if len(evictions) > 0 {
				ev = strings.Join(evictions, ", ")
			}

			fmt.Fprintf(&b, "  #%-8d  %s\n", r.BlockID, ev)
		}
	}

	return b.String()
}

// Lookup generates a report for lookup operations, showing which blocks were looked up,
// whether they were hits or misses, and their locations if they were hits.
func Lookup(results []cache.LookupResult) string {
	var b strings.Builder

	hits, misses := 0, 0
	for _, r := range results {
		if r.Hit {
			hits++
		} else {
			misses++
		}
	}

	fmt.Fprintf(&b, "LOOKUP — %d block(s)  [hits: %d  misses: %d]\n", len(results), hits, misses)
	fmt.Fprintf(&b, "%-10s  %-8s  %s\n", "Block", "Hit", "Locations")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 50))

	for _, r := range results {
		locs := "—"
		if len(r.Locations) > 0 {
			locs = strings.Join(r.Locations, ", ")
		}

		hit := "miss"
		if r.Hit {
			hit = "HIT"
		}

		fmt.Fprintf(&b, "  #%-8d  %-8s  %s\n", r.BlockID, hit, locs)
	}

	return b.String()
}

// Restore generates a report for restore operations, showing which blocks were restored,
// whether they were restored from cache or computed, the time taken for each operation,
// and a summary of the overall restore process.
func Restore(results []cache.RestoreResult, overall time.Duration, verbose bool) string {
	var b strings.Builder

	tierCounts := make(map[string]int)
	computed, restored := 0, 0
	for _, r := range results {
		if r.Computed {
			computed++
		} else {
			restored++

			if _, ok := tierCounts[r.TierName]; !ok {
				tierCounts[r.TierName] = 0
			}

			tierCounts[r.TierName]++
		}
	}

	fmt.Fprintf(&b, "RESTORE — %d block(s)  [from cache: %d  computed: %d]\n", len(results), restored, computed)
	fmt.Fprintf(&b, "%-10s  %-12s  %-10s  %s\n", "Block", "Source", "Time", "Note")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 60))

	if verbose {
		for _, r := range results {
			source := r.TierName
			var note string
			if r.ForceRecomputed {
				source = "RECOMPUTE"
				note = "found — but decided to recompute"
			} else if r.Computed {
				source = "COMPUTE"
				note = "not found — computed and auto-stored"
			} else {
				note = fmt.Sprintf("restored from %s", r.TierName)
			}

			fmt.Fprintf(&b, "  #%-8d  %-12s  %-10s  %s\n", r.BlockID, source, fmtDur(r.RestoreTime), note)
		}
	}

	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 60))

	tierDur := make(map[string]time.Duration)
	var totalCompute time.Duration
	for _, r := range results {
		if r.Computed {
			totalCompute += r.RestoreTime
		} else {
			tierDur[r.TierName] += r.RestoreTime
		}
	}

	var maxRestore time.Duration
	for _, d := range tierDur {
		if d > maxRestore {
			maxRestore = d
		}
	}

	for tierName, d := range tierDur {
		fmt.Fprintf(&b, "Lane %-14s %s (distribution: %d block(s))\n", tierName+":", fmtDur(d), tierCounts[tierName])
	}

	if totalCompute > 0 {
		fmt.Fprintf(&b, "Sequential compute:   %s  (%d block(s))\n", fmtDur(totalCompute), computed)
	}

	fmt.Fprintf(&b, "Overall time:         %s  max(parallel restore %s, compute %s)\n", fmtDur(overall), fmtDur(maxRestore), fmtDur(totalCompute))

	return b.String()
}

// Status generates a report showing the current status of each cache tier,
// including the used and total capacity, and which blocks are currently stored in each tier.
func Status(statuses []cache.TierStatus) string {
	var b strings.Builder

	fmt.Fprintf(&b, "STATUS\n")
	fmt.Fprintf(&b, "%-14s  %-10s  %s\n", "Tier", "Used/Cap", "Blocks")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 60))

	for _, s := range statuses {
		blocks := "—"

		if len(s.Blocks) > 0 {
			parts := make([]string, len(s.Blocks))

			for i, id := range s.Blocks {
				parts[i] = fmt.Sprintf("#%d", id)
			}

			blocks = strings.Join(parts, " ")
		}

		fmt.Fprintf(&b, "  %-12s  %d/%-8d  %s\n", s.Name, s.Used, s.Capacity, blocks)
	}

	return b.String()
}

func fmtDur(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	}

	return fmt.Sprintf("%.2fms", float64(d.Milliseconds()))
}
