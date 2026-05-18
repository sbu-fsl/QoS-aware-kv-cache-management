package cmd

import (
	"fmt"
	"strconv"
	"strings"
)

// parseIDs accept a blockID list and converts it to a list of integers.
func parseIDs(s string) ([]int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty blocks field")
	}

	// range form: "N-M"
	if idx := strings.Index(s, "-"); idx > 0 && !strings.Contains(s, ",") {
		parts := strings.SplitN(s, "-", 2)

		lo, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		hi, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))

		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("invalid range %q", s)
		}
		if lo > hi {
			return nil, fmt.Errorf("range start %d > end %d", lo, hi)
		}

		ids := make([]int, hi-lo+1)
		for i := range ids {
			ids[i] = lo + i
		}

		return ids, nil
	}

	// comma-separated form
	parts := strings.Split(s, ",")
	ids := make([]int, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		id, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid block ID %q", p)
		}

		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no valid block IDs in %q", s)
	}

	return ids, nil
}
