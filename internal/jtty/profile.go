package jtty

import "fmt"

// ContestProfile describes the operator-facing JTTY message profiles used by
// the WSJT-X JTTY UI. These are message templates, not the physical-layer
// coding implementation.
type ContestProfile struct {
	ID       string
	Macro    string
	Expanded string
	Frames   int
}

// WSJTXDefaultProfiles mirrors the F1-F8 JTTY macro table documented for
// WSJT-X 3.2.0-rc1. The physical encoder/decoder remains a separate layer.
var WSJTXDefaultProfiles = []ContestProfile{
	{ID: "F1", Macro: "CQ %M CQ", Expanded: "CQ K1ABC CQ", Frames: 1},
	{ID: "F2", Macro: "%H %E", Expanded: "W9XYZ 599 107", Frames: 2},
	{ID: "F3", Macro: "%H TU CQ %M CQ", Expanded: "W9XYZ TU CQ K1ABC CQ", Frames: 2},
	{ID: "F4", Macro: "%M", Expanded: "K1ABC", Frames: 1},
	{ID: "F5", Macro: "%H", Expanded: "W9XYZ", Frames: 1},
	{ID: "F6", Macro: "TU NOW %Q %E", Expanded: "TU NOW W7UVW 599 108", Frames: 2},
	{ID: "F7", Macro: "%H AGN?", Expanded: "W9XYZ AGN?", Frames: 1},
	{ID: "F8", Macro: "%E", Expanded: "599 107", Frames: 1},
}

// ExpandExchange is the default exchange format documented by WSJT-X for
// JTTY contest operation: signal report plus serial number.
func ExpandExchange(serial int) string {
	if serial < 0 {
		serial = 0
	}
	return sprintf("599 %03d", serial)
}

// Kept local to avoid pulling formatting into the macro package.
func sprintf(format string, v any) string {
	return fmt.Sprintf(format, v)
}
