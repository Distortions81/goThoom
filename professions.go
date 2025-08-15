package main

import (
	"strings"
)

// professionPictID returns the CL_Images pict ID used by the classic client
// for the given profession/class string (as parsed from be-info). Returns 0 if
// no dedicated icon should be shown.
//
// Mapping derived from old_mac_client PlayersList::PictureIDForClass().
func professionPictID(class string) uint16 {
	c := strings.ToLower(strings.TrimSpace(class))
	// Remove leading articles that may appear in localized/English text.
	if strings.HasPrefix(c, "a ") {
		c = strings.TrimSpace(c[2:])
	} else if strings.HasPrefix(c, "an ") {
		c = strings.TrimSpace(c[3:])
	} else if strings.HasPrefix(c, "the ") {
		c = strings.TrimSpace(c[4:])
	}
	// Normalize spaces
	c = strings.ReplaceAll(c, "  ", " ")

	switch c {
	case "healer":
		return 635 // moonstone
	case "fighter":
		return 1580 // greataxe
	case "mystic":
		return 417 // sunstone
	case "apprentice mystic":
		return 624 // staff
	case "journeyman mystic":
		return 1408 // orb
	case "bloodmage":
		return 2252 // bloodblade
	case "champion":
		return 1528 // fellblade
	case "ranger":
		return 127 // gossamer
	case "game master", "gm", "g.m.":
		return 4069 // GM icon (may be ARINDAL-specific)
	default:
		// "exile" and unknown types: no icon
		return 0
	}
}
