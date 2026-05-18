package cmd

import (
	"fmt"
	"strconv"
	"strings"
)

// parseIDs accept a blockID list and converts it to a list of integers.
func parseIDs(s string) ([]int, error) {
	if s == "" {
		return nil, fmt.Errorf("no block IDs provided")
	}

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
		return nil, fmt.Errorf("no valid block IDs")
	}

	return ids, nil
}
