package jtty

import (
	"regexp"
	"strings"

	"github.com/BH2VSQ/jtty-go/internal/model"
)

// Go's regexp package uses RE2 and does not support look-behind/look-ahead.
// We therefore validate token boundaries explicitly after matching.
var callsignPattern = regexp.MustCompile(`(?i)[A-Z0-9]{1,3}[0-9][A-Z]{1,4}`)

func NormalizeCallsign(call string) string { return strings.ToUpper(strings.TrimSpace(call)) }

// ExtractCallsigns is intentionally conservative. The final parser should be
// expanded using contest/JTTY message grammar rather than only regex matching.
func ExtractCallsigns(message string) []model.Callsign {
	upper := strings.ToUpper(message)
	matches := callsignPattern.FindAllStringIndex(upper, -1)
	out := make([]model.Callsign, 0, len(matches))

	for _, m := range matches {
		start, end := m[0], m[1]
		if start > 0 && isCallChar(upper[start-1]) {
			continue
		}
		if end < len(upper) && isCallChar(upper[end]) {
			continue
		}
		out = append(out, model.Callsign{
			Value:      upper[start:end],
			Start:      start,
			End:        end,
			Confidence: 0.65,
		})
	}
	return out
}

func isCallChar(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}
