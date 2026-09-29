package jtty

// Frame is a decoded/encoded JTTY message-level frame. The physical layer
// uses the 34-bit source payload plus the end-of-message marker before it is
// mapped to the 59-symbol four-tone waveform.
type Frame struct {
	Index int
	Text  string
}

// SplitArbitraryText keeps the historical five-character presentation helper.
// Normal protocol transmission should use PackMessage, which applies the
// WSJT-X source grammar and physical packing rules.
func SplitArbitraryText(text string) []Frame {
	b := []byte(text)
	if len(b) == 0 {
		return nil
	}
	frames := make([]Frame, 0, (len(b)+4)/5)
	for i, index := 0, 0; i < len(b); i, index = i+5, index+1 {
		end := i + 5
		if end > len(b) {
			end = len(b)
		}
		frames = append(frames, Frame{Index: index, Text: string(b[i:end])})
	}
	return frames
}
