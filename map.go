package main

// GetMapTiles returns a snapshot of cached map tiles for UI consumption.
func GetMapTiles() []mapTile {
	stateMu.Lock()
	defer stateMu.Unlock()
	tiles := make([]mapTile, 0, len(state.mapTiles))
	for _, t := range state.mapTiles {
		tiles = append(tiles, t)
	}
	return tiles
}
