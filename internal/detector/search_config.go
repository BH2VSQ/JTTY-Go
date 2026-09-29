package detector

// JTTY spectrum-search parameters are compile-time processing constants.
// The operator chooses only the displayed receive cutoff from the main menu.
const (
	SearchMinFrequencyHz = 0.0
	SearchMaxFrequencyHz = 4000.0
	SearchFFTSize        = 4096
	SearchHop            = 1024
)
