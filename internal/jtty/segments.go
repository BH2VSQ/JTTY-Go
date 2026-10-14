package jtty

import (
	"fmt"
	"strings"
)

// PackSegment retains the source grammar while allowing a later segment to
// continue the same over. PackMessage keeps its original final-message contract.
func PackSegment(raw string, profile ExchangeProfile, final bool) ([]PackedFrame, string, error) {
	frames, text, err := PackMessage(raw, profile)
	if err != nil {
		return nil, "", err
	}
	if !final && len(frames) > 0 {
		frames[len(frames)-1].Bits &^= 1
	}
	return frames, text, nil
}

type TransmitSegment struct {
	Text   string
	Frames []PackedFrame
	Final  bool
}

// PackTransmitLine splits long lines at word boundaries. Newlines delimit
// messages; ordinary length boundaries do not emit EOM. A trailing TEXT5 space
// preserves the separator that the fixed-width Fortran packer trims.
func PackTransmitLine(raw string, profile ExchangeProfile) ([]TransmitSegment, string, error) {
	var segments []TransmitSegment
	var canonicalLines []string
	raw = strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	for _, line := range strings.Split(raw, "\n") {
		normalized := strings.TrimSpace(normalizeJTTYMessage(line))
		if normalized == "" {
			continue
		}
		var textParts []string
		for len(normalized) > 0 {
			end := len(normalized)
			if end > 80 {
				end = 80
				if normalized[end] != ' ' {
					if space := strings.LastIndexByte(normalized[:end], ' '); space > 0 {
						end = space
					}
				}
			}
			part := strings.TrimRight(normalized[:end], " ")
			rest := normalized[end:]
			boundarySpace := strings.HasPrefix(rest, " ")
			rest = strings.TrimLeft(rest, " ")
			final := rest == ""
			frames, text, err := PackSegment(part, profile, final)
			if err != nil {
				return nil, "", fmt.Errorf("JTTY 分段无法编码: %w", err)
			}
			if !final && boundarySpace && len(frames) > 0 {
				atom, _, ok := unpackAtom(frames[len(frames)-1])
				if !ok {
					return nil, "", fmt.Errorf("invalid segment boundary")
				}
				if atom.Kind == atomText5 && !strings.HasSuffix(atom.Text, " ") {
					if len(atom.Text) < 5 {
						atom.Text += " "
						frame, ok := packAtom(atom, false)
						if !ok {
							return nil, "", fmt.Errorf("invalid separator")
						}
						frames[len(frames)-1] = frame
					} else {
						frame, _ := packAtom(SourceAtom{Kind: atomText5, Text: " "}, false)
						frames = append(frames, frame)
					}
				}
			}
			segments = append(segments, TransmitSegment{Text: text, Frames: frames, Final: final})
			textParts = append(textParts, text)
			if !final && boundarySpace {
				textParts = append(textParts, " ")
			}
			normalized = rest
		}
		canonicalLines = append(canonicalLines, strings.Join(textParts, ""))
	}
	return segments, strings.Join(canonicalLines, "\n"), nil
}
