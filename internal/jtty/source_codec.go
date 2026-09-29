package jtty

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// This file mirrors the message/source grammar in WSJT-X 3.2.0-rc1
// lib/jtty/jtty_source_codec.f90 and lib/jtty/jtty_mod.f90.
// It intentionally implements the current STRUCT30 grammar, not the older
// literal 599+TEXT5 type-2 format.

const MaxFrames = 16

const (
	atomCall = iota
	atomExchNum
	atomExchLoc
	atomExchPair
	atomExchNumTime
	atomControl
	atomGrid4
	atomText5
)

const (
	callCQ = iota
	callCall
	callTUCQ
	callCallTU
	callCallAGN
	callTUNow
)

const (
	roleFieldOnly = iota
	roleFull
)

const (
	numSerial = iota
	numCQZone
	numITUZone
	numAge
	numPower
	numCheck
	numLicenseYear
	numGeneric
)

const (
	locStateProvince = iota
	locARRLRACSection
	locCountryPrefix
	locQTH
	locAdmin
)

const (
	pairZoneLoc3 = iota
	pairClassSection
)

const (
	miscControl = iota
	miscGrid4
)

const (
	controlAGN = iota
	controlCALL
	controlAGNCall
	controlNR
	controlAGNnr
	controlEXCH
	controlSTATE
	controlSECTION
	controlZONE
	controlGRID
	controlRPRT
	controlQSLTU
	controlTU
	controlQRZ
	controlQSOB4
	controlWAIT
	controlNIL
	controlOK
)

var sourceAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ +-./?!\"#$%,&*()_'=[]{}<>|:;"

var controlText = [...]string{
	"AGN?", "CALL?", "AGN CALL", "NR?", "AGN NR", "EXCH?", "STATE?", "SECTION?",
	"ZONE?", "GRID?", "RPRT?", "QSL TU", "TU", "QRZ?", "QSO B4", "WAIT", "NIL?", "OK?",
}

// All 86 section names from packjt77_grammar.f90, 1-based in the reference.
var arrlSections = []string{
	"AB ", "AK ", "AL ", "AR ", "AZ ", "BC ", "CO ", "CT ", "DE ", "EB ",
	"EMA", "ENY", "EPA", "EWA", "GA ", "GH ", "IA ", "ID ", "IL ", "IN ",
	"KS ", "KY ", "LA ", "LAX", "NS ", "MB ", "MDC", "ME ", "MI ", "MN ",
	"MO ", "MS ", "MT ", "NC ", "ND ", "NE ", "NFL", "NH ", "NL ", "NLI",
	"NM ", "NNJ", "NNY", "TER", "NTX", "NV ", "OH ", "OK ", "ONE", "ONN",
	"ONS", "OR ", "ORG", "PAC", "PR ", "QC ", "RI ", "SB ", "SC ", "SCV",
	"SD ", "SDG", "SF ", "SFL", "SJV", "SK ", "SNJ", "STX", "SV ", "TN ",
	"UT ", "VA ", "VI ", "VT ", "WCF", "WI ", "WMA", "WNY", "WPA", "WTX",
	"WV ", "WWA", "WY ", "DX ", "PE ", "NB ",
}

// SourceAtom corresponds to jtty_source_atom in WSJT-X.
type SourceAtom struct {
	Kind    int
	Subtype int
	Role    int
	Value   int
	Value2  int
	Text    string
}

type ExchangeProfile int

const (
	ExchangeUnknown ExchangeProfile = iota
	ExchangeFieldDay
	ExchangeRTTYRoundup
)

type PackedFrame struct {
	Bits uint64 // low 34 bits used, bit 33 is reserved zero, bit 34 is EOM.
}

func bit34(v uint64) uint64 { return v & ((uint64(1) << 34) - 1) }

func normalizeJTTYMessage(raw string) string {
	raw = strings.TrimRight(raw, "\x00")
	raw = strings.ReplaceAll(raw, "~", " ")
	raw = strings.ToUpper(raw)
	var b strings.Builder
	lastSpace := true
	for _, r := range raw {
		c := string(r)
		if sourceIndex(c) < 0 {
			if c != " " {
				c = "#"
			}
		}
		if c == " " {
			if lastSpace {
				continue
			}
			b.WriteByte(' ')
			lastSpace = true
			continue
		}
		b.WriteString(c)
		lastSpace = false
	}
	return b.String()
}

func sourceIndex(c string) int {
	if len(c) != 1 {
		return -1
	}
	return strings.IndexByte(sourceAlphabet, c[0])
}

func sourceChar(i int) byte {
	if i < 0 || i >= len(sourceAlphabet) {
		return '#'
	}
	return sourceAlphabet[i]
}

func standardCall(call string) bool {
	call = strings.ToUpper(strings.TrimSpace(call))
	n := len(call)
	if n < 3 || n > 6 || call[0] == 'Q' || strings.Contains(call, "/") {
		return false
	}
	area := 0
	for i := n - 1; i >= 1; i-- {
		if call[i] >= '0' && call[i] <= '9' {
			area = i + 1 // 1-based position from the Fortran implementation
			break
		}
	}
	if area != 2 && area != 3 {
		return false
	}
	letters, digits := 0, 0
	for i := 0; i < area-1; i++ {
		switch {
		case call[i] >= 'A' && call[i] <= 'Z':
			letters++
		case call[i] >= '0' && call[i] <= '9':
			digits++
		default:
			return false
		}
	}
	if letters == 0 || digits >= area-1 {
		return false
	}
	suffix := call[area:]
	if len(suffix) < 1 || len(suffix) > 3 {
		return false
	}
	for i := range suffix {
		if suffix[i] < 'A' || suffix[i] > 'Z' {
			return false
		}
	}
	// A round trip through c28 is the final predicate in the reference.
	decoded, ok := unpack28Standard(pack28Standard(call))
	return ok && decoded == call
}

func pack28Standard(call string) uint32 {
	call = strings.ToUpper(strings.TrimSpace(call))
	area := 0
	for i := len(call) - 1; i >= 1; i-- {
		if call[i] >= '0' && call[i] <= '9' {
			area = i + 1
			break
		}
	}
	callsign := ""
	if area == 2 {
		callsign = " " + call
	} else {
		callsign = call
	}
	callsign = (callsign + "      ")[:6]
	a1 := " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	a2 := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	a3 := "0123456789"
	a4 := " ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	i1 := strings.IndexByte(a1, callsign[0])
	i2 := strings.IndexByte(a2, callsign[1])
	i3 := strings.IndexByte(a3, callsign[2])
	i4 := strings.IndexByte(a4, callsign[3])
	i5 := strings.IndexByte(a4, callsign[4])
	i6 := strings.IndexByte(a4, callsign[5])
	if i1 < 0 || i2 < 0 || i3 < 0 || i4 < 0 || i5 < 0 || i6 < 0 {
		return 0
	}
	n := uint64(36*10*27*27*27*i1 + 10*27*27*27*i2 + 27*27*27*i3 + 27*27*i4 + 27*i5 + i6)
	n += 2063592 + 4194304
	return uint32(n & ((1 << 28) - 1))
}

func unpack28Standard(n28 uint32) (string, bool) {
	const tokens = 2063592
	const max22 = 4194304
	if uint64(n28) < tokens+max22 {
		return "", false
	}
	n := int(uint64(n28) - tokens - max22)
	i1 := n / (36 * 10 * 27 * 27 * 27)
	n -= i1 * (36 * 10 * 27 * 27 * 27)
	i2 := n / (10 * 27 * 27 * 27)
	n -= i2 * (10 * 27 * 27 * 27)
	i3 := n / (27 * 27 * 27)
	n -= i3 * (27 * 27 * 27)
	i4 := n / (27 * 27)
	n -= i4 * (27 * 27)
	i5 := n / 27
	i6 := n - i5*27
	a1 := " 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	a2 := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	a3 := "0123456789"
	a4 := " ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if i1 >= len(a1) || i2 >= len(a2) || i3 >= len(a3) || i4 >= len(a4) || i5 >= len(a4) || i6 >= len(a4) {
		return "", false
	}
	call := strings.TrimLeft(string([]byte{a1[i1], a2[i2], a3[i3], a4[i4], a4[i5], a4[i6]}), " ")
	return call, standardCallShapeOnly(call)
}

func standardCallShapeOnly(call string) bool {
	n := len(call)
	if n < 3 || n > 6 || call[0] == 'Q' {
		return false
	}
	area := 0
	for i := n - 1; i >= 1; i-- {
		if call[i] >= '0' && call[i] <= '9' {
			area = i + 1
			break
		}
	}
	if area != 2 && area != 3 {
		return false
	}
	letters, digits := 0, 0
	for i := 0; i < area-1; i++ {
		if call[i] >= 'A' && call[i] <= 'Z' {
			letters++
		} else if call[i] >= '0' && call[i] <= '9' {
			digits++
		} else {
			return false
		}
	}
	if letters == 0 || digits >= area-1 {
		return false
	}
	suffix := call[area:]
	if len(suffix) < 1 || len(suffix) > 3 {
		return false
	}
	for i := range suffix {
		if suffix[i] < 'A' || suffix[i] > 'Z' {
			return false
		}
	}
	return true
}

func packAtom(atom SourceAtom, eom bool) (PackedFrame, bool) {
	// WSJT-X constructs a 32-bit grammar word first and then appends the
	// reserved bit (bit 33, always zero) and EOM (bit 34):
	//     frame34 = grammar32<<2 | eom
	// The in-memory Bits representation uses bit 0 as EOM, bit 1 as the
	// reserved zero, and bits 2..33 as grammar bits 1..32.
	grammar32 := uint64(0)
	switch atom.Kind {
	case atomCall:
		if atom.Subtype < 0 || atom.Subtype > 5 || !standardCall(atom.Text) {
			return PackedFrame{}, false
		}
		n28 := uint64(pack28Standard(atom.Text))
		var i2, n2 uint64
		if atom.Subtype <= 3 {
			i2 = 0
			n2 = uint64(atom.Subtype)
		} else {
			i2 = 1
			n2 = uint64(atom.Subtype - 4)
		}
		grammar32 = (n28 << 4) | (n2 << 2) | i2
	case atomExchNum:
		if !validRole(atom.Role) || !validNumber(atom.Subtype, atom.Value) {
			return PackedFrame{}, false
		}
		body := (uint64(atom.Role) << 26) | (uint64(atom.Subtype) << 22) | (uint64(atom.Value) << 5)
		grammar32 = (body << 5) | 2
	case atomExchLoc:
		if !validRole(atom.Role) || atom.Subtype < 0 || atom.Subtype > 4 {
			return PackedFrame{}, false
		}
		tokenValue, ok := packBase36(atom.Text)
		if !ok {
			return PackedFrame{}, false
		}
		n := len(atom.Text)
		body := (uint64(atom.Role) << 26) | (uint64(atom.Subtype) << 22) | (uint64(n-2) << 21) | (tokenValue << 5)
		grammar32 = (body << 5) | (1 << 2) | 2
	case atomExchPair:
		var pair uint64
		switch atom.Subtype {
		case pairZoneLoc3:
			if atom.Value < 1 || atom.Value > 40 {
				return PackedFrame{}, false
			}
			tokenValue, ok := packBase36(atom.Text)
			if !ok {
				return PackedFrame{}, false
			}
			pair = (uint64(atom.Value) << 17) | (uint64(len(atom.Text)-2) << 16) | tokenValue
		case pairClassSection:
			if atom.Value < 1 || atom.Value > 32 || atom.Value2 < 1 || atom.Value2 > len(arrlSections) || len(atom.Text) != 1 {
				return PackedFrame{}, false
			}
			ci := int(atom.Text[0] - 'A')
			if ci < 0 || ci > 5 {
				return PackedFrame{}, false
			}
			pair = (uint64(atom.Value) << 17) | (uint64(ci) << 14) | (uint64(atom.Value2) << 7)
		default:
			return PackedFrame{}, false
		}
		body := (uint64(atom.Subtype) << 24) | (pair << 1)
		grammar32 = (body << 5) | (2 << 2) | 2
	case atomExchNumTime:
		if !validRole(atom.Role) || atom.Value < 0 || atom.Value > 16383 || atom.Value2 < 0 || atom.Value2 > 1439 {
			return PackedFrame{}, false
		}
		body := (uint64(atom.Role) << 26) | (uint64(atom.Value) << 12) | (uint64(atom.Value2) << 1)
		grammar32 = (body << 5) | (3 << 2) | 2
	case atomControl:
		if atom.Subtype < 0 || atom.Subtype > 17 {
			return PackedFrame{}, false
		}
		body := (uint64(miscControl) << 23) | (uint64(atom.Subtype) << 16)
		grammar32 = (body << 5) | (4 << 2) | 2
	case atomGrid4:
		if !validRole(atom.Role) {
			return PackedFrame{}, false
		}
		idx, ok := grid4ToIndex(atom.Text)
		if !ok {
			return PackedFrame{}, false
		}
		data := (uint64(atom.Role) << 22) | (uint64(idx) << 7)
		body := (uint64(miscGrid4) << 23) | data
		grammar32 = (body << 5) | (4 << 2) | 2
	case atomText5:
		if len(atom.Text) == 0 || len(atom.Text) > 5 {
			return PackedFrame{}, false
		}
		text := atom.Text + strings.Repeat(" ", 5-len(atom.Text))
		var top30 uint64
		for i := 0; i < 5; i++ {
			idx := sourceIndex(text[i : i+1])
			if idx < 0 {
				return PackedFrame{}, false
			}
			top30 = top30*64 + uint64(idx)
		}
		grammar32 = (top30 << 2) | 3
	default:
		return PackedFrame{}, false
	}
	return PackedFrame{Bits: bit34((grammar32 << 2) | boolBit(eom))}, true
}

func boolBit(v bool) uint64 {
	if v {
		return 1
	}
	return 0
}

func unpackAtom(frame PackedFrame) (atom SourceAtom, eom bool, valid bool) {
	bits := bit34(frame.Bits)
	eom = bits&1 != 0
	if (bits>>1)&1 != 0 || bits == 0 {
		return SourceAtom{}, eom, false
	}
	grammar32 := bits >> 2
	top30 := grammar32 >> 2
	i2 := int(grammar32 & 3)
	switch i2 {
	case 0, 1:
		n28 := uint32(grammar32 >> 4)
		n2 := int((grammar32 >> 2) & 3)
		if i2 == 1 && n2 > 1 {
			return SourceAtom{}, eom, false
		}
		call, ok := unpack28Standard(n28)
		if !ok {
			return SourceAtom{}, eom, false
		}
		action := n2
		if i2 == 1 {
			action = n2 + 4
		}
		return SourceAtom{Kind: atomCall, Subtype: action, Text: call}, eom, true
	case 2:
		family := int((grammar32 >> 2) & 7)
		body := grammar32 >> 5
		switch family {
		case 0:
			atom := SourceAtom{Kind: atomExchNum, Role: int((body >> 26) & 1), Subtype: int((body >> 22) & 15), Value: int((body >> 5) & ((1 << 17) - 1))}
			if (body&31) != 0 || !validNumber(atom.Subtype, atom.Value) {
				return SourceAtom{}, eom, false
			}
			return atom, eom, true
		case 1:
			role := int((body >> 26) & 1)
			kind := int((body >> 22) & 15)
			n := int((body>>21)&1) + 2
			token := (body >> 5) & ((1 << 16) - 1)
			if body&31 != 0 || kind > 4 {
				return SourceAtom{}, eom, false
			}
			text, ok := unpackBase36(token, n)
			if !ok {
				return SourceAtom{}, eom, false
			}
			return SourceAtom{Kind: atomExchLoc, Role: role, Subtype: kind, Text: text}, eom, true
		case 2:
			subtype := int((body >> 24) & 7)
			pairData := (body >> 1) & ((1 << 23) - 1)
			if body&1 != 0 {
				return SourceAtom{}, eom, false
			}
			switch subtype {
			case pairZoneLoc3:
				zone := int((pairData >> 17) & 63)
				n := int((pairData>>16)&1) + 2
				token := pairData & ((1 << 16) - 1)
				if zone < 1 || zone > 40 {
					return SourceAtom{}, eom, false
				}
				text, ok := unpackBase36(token, n)
				if !ok {
					return SourceAtom{}, eom, false
				}
				return SourceAtom{Kind: atomExchPair, Subtype: subtype, Value: zone, Text: text}, eom, true
			case pairClassSection:
				count := int((pairData >> 17) & 63)
				classIndex := int((pairData >> 14) & 7)
				sectionIndex := int((pairData >> 7) & 127)
				if count < 1 || count > 32 || classIndex > 5 || sectionIndex < 1 || sectionIndex > len(arrlSections) || pairData&127 != 0 {
					return SourceAtom{}, eom, false
				}
				return SourceAtom{Kind: atomExchPair, Subtype: subtype, Value: count, Value2: sectionIndex, Text: string(byte('A' + classIndex))}, eom, true
			}
		case 3:
			role := int((body >> 26) & 1)
			serial := int((body >> 12) & 16383)
			minute := int((body >> 1) & 2047)
			if body&1 != 0 || minute > 1439 {
				return SourceAtom{}, eom, false
			}
			return SourceAtom{Kind: atomExchNumTime, Role: role, Value: serial, Value2: minute}, eom, true
		case 4:
			misc := int((body >> 23) & 15)
			data := body & ((1 << 23) - 1)
			switch misc {
			case miscControl:
				subtype := int((data >> 16) & 127)
				if data&0xFFFF != 0 || subtype > 17 {
					return SourceAtom{}, eom, false
				}
				return SourceAtom{Kind: atomControl, Subtype: subtype}, eom, true
			case miscGrid4:
				role := int((data >> 22) & 1)
				idx := int((data >> 7) & 0x7FFF)
				if data&127 != 0 || idx >= 32400 {
					return SourceAtom{}, eom, false
				}
				grid, ok := indexToGrid4(idx)
				if !ok {
					return SourceAtom{}, eom, false
				}
				return SourceAtom{Kind: atomGrid4, Role: role, Text: grid}, eom, true
			}
		}
	case 3:
		var text strings.Builder
		for i := 0; i < 5; i++ {
			idx := int((top30 >> uint(6*(4-i))) & 63)
			text.WriteByte(sourceChar(idx))
		}
		return SourceAtom{Kind: atomText5, Text: text.String()}, eom, true
	}
	return SourceAtom{}, eom, false
}

func renderAtom(atom SourceAtom) (string, bool) {
	switch atom.Kind {
	case atomCall:
		switch atom.Subtype {
		case callCQ:
			return "CQ " + atom.Text + " CQ", true
		case callCall:
			return atom.Text, true
		case callTUCQ:
			return "TU " + atom.Text + " CQ", true
		case callCallTU:
			return atom.Text + " TU", true
		case callCallAGN:
			return atom.Text + " AGN?", true
		case callTUNow:
			return "TU NOW " + atom.Text, true
		}
	case atomExchNum:
		if !validRole(atom.Role) || !validNumber(atom.Subtype, atom.Value) {
			return "", false
		}
		field := renderNumber(atom.Subtype, atom.Value)
		if atom.Role == roleFull {
			return "599 " + field, true
		}
		return field, true
	case atomExchLoc:
		if atom.Subtype < 0 || atom.Subtype > 4 || !validRole(atom.Role) {
			return "", false
		}
		if atom.Role == roleFull {
			return "599 " + strings.TrimSpace(atom.Text), true
		}
		return strings.TrimSpace(atom.Text), true
	case atomExchPair:
		switch atom.Subtype {
		case pairZoneLoc3:
			return fmt.Sprintf("599 %02d %s", atom.Value, atom.Text), true
		case pairClassSection:
			if atom.Value2 < 1 || atom.Value2 > len(arrlSections) || len(atom.Text) != 1 {
				return "", false
			}
			return fmt.Sprintf("%d%s %s", atom.Value, atom.Text, strings.TrimSpace(arrlSections[atom.Value2-1])), true
		}
	case atomExchNumTime:
		if !validRole(atom.Role) || atom.Value < 0 || atom.Value > 16383 || atom.Value2 < 0 || atom.Value2 > 1439 {
			return "", false
		}
		field := fmt.Sprintf("%03d %02d%02d", atom.Value, atom.Value2/60, atom.Value2%60)
		if atom.Role == roleFull {
			return "599 " + field, true
		}
		return field, true
	case atomControl:
		if atom.Subtype >= 0 && atom.Subtype < len(controlText) {
			return controlText[atom.Subtype], true
		}
	case atomGrid4:
		if atom.Role == roleFull {
			return "599 " + atom.Text, true
		}
		return atom.Text, true
	case atomText5:
		return atom.Text, true
	}
	return "", false
}

func renderNumber(kind, value int) string {
	switch kind {
	case numSerial, numCQZone, numITUZone, numCheck:
		if kind == numSerial {
			if value < 1000 {
				return fmt.Sprintf("%03d", value)
			}
			return strconv.Itoa(value)
		}
		if value < 100 {
			return fmt.Sprintf("%02d", value)
		}
		return strconv.Itoa(value)
	case numLicenseYear:
		return fmt.Sprintf("%04d", value)
	default:
		return strconv.Itoa(value)
	}
}

func validRole(role int) bool { return role == roleFieldOnly || role == roleFull }

func validNumber(kind, value int) bool {
	if kind < 0 || kind > 7 || value < 0 || value > 131071 {
		return false
	}
	switch kind {
	case numCQZone:
		return value >= 1 && value <= 40
	case numITUZone:
		return value >= 1 && value <= 90
	case numLicenseYear:
		return value <= 9999
	}
	return true
}

func packBase36(token string) (uint64, bool) {
	token = strings.ToUpper(strings.TrimSpace(token))
	if len(token) != 2 && len(token) != 3 {
		return 0, false
	}
	var value uint64
	for i := range token {
		c := token[i]
		var d byte
		switch {
		case c >= '0' && c <= '9':
			d = c - '0'
		case c >= 'A' && c <= 'Z':
			d = c - 'A' + 10
		default:
			return 0, false
		}
		value = value*36 + uint64(d)
	}
	if len(token) == 3 && value < 36*36 {
		return 0, false
	}
	return value, true
}

func unpackBase36(value uint64, n int) (string, bool) {
	if n != 2 && n != 3 {
		return "", false
	}
	limit := uint64(36 * 36)
	if n == 2 && value >= limit {
		return "", false
	}
	if n == 3 && (value < limit || value >= 36*36*36) {
		return "", false
	}
	out := make([]byte, n)
	for i := n - 1; i >= 0; i-- {
		d := byte(value % 36)
		value /= 36
		if d < 10 {
			out[i] = '0' + d
		} else {
			out[i] = 'A' + d - 10
		}
	}
	return string(out), true
}

func grid4ToIndex(grid string) (int, bool) {
	grid = strings.ToUpper(strings.TrimSpace(grid))
	if len(grid) != 4 || grid[0] < 'A' || grid[0] > 'R' || grid[1] < 'A' || grid[1] > 'R' || grid[2] < '0' || grid[2] > '9' || grid[3] < '0' || grid[3] > '9' {
		return 0, false
	}
	a := int(grid[0] - 'A')
	b := int(grid[1] - 'A')
	c := int(grid[2] - '0')
	d := int(grid[3] - '0')
	return ((a*18+b)*10+c)*10 + d, true
}

func indexToGrid4(index int) (string, bool) {
	if index < 0 || index >= 32400 {
		return "", false
	}
	work := index
	d := work % 10
	work /= 10
	c := work % 10
	work /= 10
	b := work % 18
	a := work / 18
	return string([]byte{byte('A' + a), byte('A' + b), byte('0' + c), byte('0' + d)}), true
}

func atomKey(a SourceAtom) int {
	return 100*a.Kind + 2*a.Subtype + a.Role
}

func offerCandidate(msg string, pos int, atom SourceAtom) (next int, ok bool) {
	frame, valid := packAtom(atom, false)
	if !valid {
		return 0, false
	}
	decoded, _, valid := unpackAtom(frame)
	if !valid {
		return 0, false
	}
	rendered, valid := renderAtom(decoded)
	if !valid || len(rendered) == 0 || pos+len(rendered) > len(msg) {
		return 0, false
	}
	if msg[pos:pos+len(rendered)] != rendered {
		return 0, false
	}
	end := pos + len(rendered)
	if end < len(msg) {
		if msg[end] != ' ' {
			return 0, false
		}
		return end + 1, true
	}
	return end, true
}

func decimalValue(text string) (int, bool) {
	if len(text) < 1 || len(text) > 6 {
		return 0, false
	}
	for i := range text {
		if text[i] < '0' || text[i] > '9' {
			return 0, false
		}
	}
	v, _ := strconv.Atoi(text)
	return v, v <= 131071
}

func normalizeRTTYSerials(msg string) (string, bool) {
	words := strings.Fields(msg)
	if len(words) == 0 {
		return msg, true
	}
	out := append([]string(nil), words...)
	for i := 0; i+1 < len(out); i++ {
		if out[i] != "599" {
			continue
		}
		v, ok := decimalValue(out[i+1])
		if !ok {
			continue
		}
		// Mirror WSJT-X's RTTY Roundup canonicalization: at least three digits.
		out[i+1] = fmt.Sprintf("%03d", v)
		if v >= 1000 {
			out[i+1] = strconv.Itoa(v)
		}
	}
	normalized := strings.Join(out, " ")
	if len(normalized) > 80 {
		return "", false
	}
	return normalized, true
}

func PackMessage(raw string, profile ExchangeProfile) ([]PackedFrame, string, error) {
	msg := normalizeJTTYMessage(raw)
	if profile == ExchangeRTTYRoundup {
		var ok bool
		msg, ok = normalizeRTTYSerials(msg)
		if !ok {
			return nil, "", errors.New("JTTY message exceeds 80 characters after RTTY Roundup normalization")
		}
	}
	if msg == "" {
		return nil, "", nil
	}
	n := len(msg)
	const inf = 999
	dp := make([]int, n+1)
	succ := make([]int, n)
	choice := make([]SourceAtom, n)
	for i := range dp {
		dp[i] = inf
	}
	dp[n] = 0

	for pos := n - 1; pos >= 0; pos-- {
		end := pos + 5
		if end > n {
			end = n
		}
		txt := SourceAtom{Kind: atomText5, Text: msg[pos:end]}
		consider := func(atom SourceAtom, next int) {
			if next < pos+1 || next > n || dp[next] == inf {
				return
			}
			cost := 1 + dp[next]
			if cost > MaxFrames || cost > dp[pos] {
				return
			}
			rank := 0
			if atom.Kind == atomText5 {
				rank = 1
			}
			bestRank := 1
			if choice[pos].Kind != atomText5 {
				bestRank = 0
			}
			key := atomKey(atom)
			bestKey := atomKey(choice[pos])
			if cost == dp[pos] {
				if rank > bestRank || (rank == bestRank && next < succ[pos]) || (rank == bestRank && next == succ[pos] && key >= bestKey) {
					return
				}
			}
			dp[pos] = cost
			succ[pos] = next
			choice[pos] = atom
		}
		consider(txt, end)

		if pos > 0 && msg[pos-1] != ' ' {
			continue
		}
		words := make([]string, 0, 3)
		cursor := pos
		for len(words) < 3 && cursor < n {
			k := strings.IndexByte(msg[cursor:], ' ')
			if k < 0 {
				k = n - cursor
			}
			if k == 0 {
				break
			}
			words = append(words, msg[cursor:cursor+k])
			cursor += k
			if cursor < n && msg[cursor] == ' ' {
				cursor++
			}
		}

		for _, w := range words {
			for action := 0; action <= 5; action++ {
				if next, ok := offerCandidate(msg, pos, SourceAtom{Kind: atomCall, Subtype: action, Text: w}); ok {
					consider(SourceAtom{Kind: atomCall, Subtype: action, Text: w}, next)
				}
			}
		}
		for id := range controlText {
			if next, ok := offerCandidate(msg, pos, SourceAtom{Kind: atomControl, Subtype: id}); ok {
				consider(SourceAtom{Kind: atomControl, Subtype: id}, next)
			}
		}

		for _, role := range []int{roleFieldOnly, roleFull} {
			field := 0
			if role == roleFull {
				if len(words) < 2 || words[0] != "599" {
					continue
				}
				field = 1
			}
			if field >= len(words) {
				continue
			}
			word := words[field]
			if value, ok := decimalValue(word); ok {
				atom := SourceAtom{Kind: atomExchNum, Role: role, Subtype: numGeneric, Value: value}
				if next, ok := offerCandidate(msg, pos, atom); ok {
					consider(atom, next)
				}
				if profile == ExchangeRTTYRoundup && role == roleFull {
					serial := SourceAtom{Kind: atomExchNum, Role: role, Subtype: numSerial, Value: value}
					if next, ok := offerCandidate(msg, pos, serial); ok {
						consider(serial, next)
					}
				}
			}
			if len(word) == 4 {
				if _, ok := grid4ToIndex(word); ok {
					atom := SourceAtom{Kind: atomGrid4, Role: role, Text: word}
					if next, ok := offerCandidate(msg, pos, atom); ok {
						consider(atom, next)
					}
				}
			}
			if role == roleFull && len(word) >= 2 && len(word) <= 3 {
				hasLetter := false
				for i := range word {
					if word[i] >= 'A' && word[i] <= 'Z' {
						hasLetter = true
						break
					}
				}
				if hasLetter {
					kinds := []int{locQTH}
					if profile == ExchangeRTTYRoundup {
						kinds = append(kinds, locStateProvince)
					}
					for _, kind := range kinds {
						atom := SourceAtom{Kind: atomExchLoc, Role: role, Subtype: kind, Text: word}
						if next, ok := offerCandidate(msg, pos, atom); ok {
							consider(atom, next)
						}
					}
				}
			}
		}

		if len(words) >= 2 {
			w0 := words[0]
			if len(w0) >= 2 {
				numeric, ok := decimalValue(w0[:len(w0)-1])
				if ok {
					class := w0[len(w0)-1]
					if class >= 'A' && class <= 'F' {
						if section := sectionIndex(words[1]); section > 0 {
							atom := SourceAtom{Kind: atomExchPair, Subtype: pairClassSection, Value: numeric, Value2: section, Text: string(class)}
							if next, ok := offerCandidate(msg, pos, atom); ok {
								consider(atom, next)
							}
						}
					}
				}
			}
		}
	}

	if dp[0] == inf || dp[0] > MaxFrames {
		return nil, "", errors.New("JTTY message cannot be packed into 16 frames")
	}
	frames := make([]PackedFrame, 0, dp[0])
	for pos := 0; pos < n; {
		atom := choice[pos]
		frame, ok := packAtom(atom, succ[pos] == n)
		if !ok {
			return nil, "", errors.New("internal JTTY source packing failure")
		}
		frames = append(frames, frame)
		pos = succ[pos]
	}
	// Round-trip the selected grammar to guarantee the same contract as WSJT-X.
	decoded, _, _, ok := UnpackMessage(frames)
	if !ok || decoded != msg {
		return nil, "", fmt.Errorf("JTTY source codec round-trip mismatch: got %q want %q", decoded, msg)
	}
	return frames, msg, nil
}

func UnpackMessage(frames []PackedFrame) (message string, trailingSep bool, lastFrame bool, valid bool) {
	if len(frames) < 1 || len(frames) > MaxFrames {
		return "", false, false, false
	}
	var b strings.Builder
	allValid := true
	lastSep := false
	lastFlag := false
	for _, frame := range frames {
		atom, eom, ok := unpackAtom(frame)
		if !ok {
			allValid = false
			continue
		}
		text, ok := renderAtom(atom)
		if !ok {
			allValid = false
			continue
		}
		lastFlag = eom
		b.WriteString(text)
		if atom.Kind == atomText5 {
			// TEXT5 may end with a real space. Preserve that boundary through
			// the frame-level metadata so the message assembler can restore it
			// after a frame split. The visible message is still trimmed below.
			lastSep = strings.HasSuffix(text, " ")
		} else {
			b.WriteByte(' ')
			lastSep = true
		}
	}
	return strings.TrimRight(b.String(), " "), lastSep, lastFlag && allValid, allValid
}

func displayJTTYText(s string) string {
	s = strings.ReplaceAll(s, "~~~~~", " ... ")
	s = strings.ReplaceAll(s, "~", " ")
	return strings.TrimLeft(s, " ")
}

func sectionIndex(section string) int {
	section = strings.ToUpper(strings.TrimSpace(section))
	for i, v := range arrlSections {
		if strings.TrimSpace(v) == section {
			return i + 1
		}
	}
	return -1
}
