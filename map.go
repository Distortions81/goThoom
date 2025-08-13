package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type AreaID string

// AreaAdjacency records the offset between two connected areas.
type AreaAdjacency struct {
	FromAreaID AreaID
	ToAreaID   AreaID
	OffsetX    int
	OffsetY    int
}

var (
	adjMu       sync.Mutex
	adjacencies []AreaAdjacency
)

// backgroundKey generates a stable identifier for the current set of
// background tiles. The key is used as an AreaID.
func backgroundKey(pics []framePicture) AreaID {
	ids := make([]string, 0)
	for _, p := range pics {
		if p.Background {
			ids = append(ids, fmt.Sprintf("%d@%d,%d", p.PictID, p.H, p.V))
		}
	}
	sort.Strings(ids)
	return AreaID(strings.Join(ids, "|"))
}

// addAdjacency records an offset between two areas.
func addAdjacency(from, to AreaID, dx, dy int) {
	if from == "" || to == "" || from == to {
		return
	}
	adjMu.Lock()
	defer adjMu.Unlock()
	for _, a := range adjacencies {
		if a.FromAreaID == from && a.ToAreaID == to {
			return
		}
	}
	adjacencies = append(adjacencies, AreaAdjacency{
		FromAreaID: from,
		ToAreaID:   to,
		OffsetX:    dx,
		OffsetY:    dy,
	})
}
