package model

import "github.com/BH2VSQ/jtty-go/internal/logbook"

type AudioSettings struct {
	InputDeviceID   string `json:"inputDeviceId"`
	OutputDeviceID  string `json:"outputDeviceId"`
	SampleRate      int    `json:"sampleRate"`
	InputChannel    string `json:"inputChannel"`
	OutputChannel   string `json:"outputChannel"`
	BufferMS        int    `json:"bufferMs"`
	TxAudioLevel    int    `json:"txAudioLevel"`
	RecordDirectory string `json:"recordDirectory"`
}

type RadioSettings struct {
	Backend         string `json:"backend"`
	RigName         string `json:"rigName"`
	RigModelID      int    `json:"rigModelId"`
	RigctldPath     string `json:"rigctldPath"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	SerialPort      string `json:"serialPort"`
	Baud            int    `json:"baud"`
	DataBits        int    `json:"dataBits"`
	StopBits        int    `json:"stopBits"`
	Handshake       string `json:"handshake"`
	PTTMethod       string `json:"pttMethod"`
	PTTSerialPort   string `json:"pttSerialPort"`
	SplitMode       string `json:"splitMode"`
	TXAudioSource   string `json:"txAudioSource"`
	ForceDTR        string `json:"forceDtr"`
	ForceRTS        string `json:"forceRts"`
	PollIntervalSec int    `json:"pollIntervalSec"`
	Mode            string `json:"mode"`
	PassbandHz      int    `json:"passbandHz"`
	ReadPowerSWR    bool   `json:"readPowerSWR"`
	HaltOnSWR       bool   `json:"haltOnSWR"`
}

type FrequencyBandSettings struct {
	Band        string  `json:"band"`
	Frequencies []int64 `json:"frequencies"`
	DefaultHz   int64   `json:"defaultHz"`
}

type WaterfallSettings struct {
	MinHz float64 `json:"minHz"`
	MaxHz float64 `json:"maxHz"`
	FFT   int     `json:"fft"`
	Hop   int     `json:"hop"`
}

type DecoderSettings struct {
	Threads              int     `json:"threads"`
	Mode                 string  `json:"mode"`
	ThresholdDb          float64 `json:"thresholdDb"`
	FrequencyToleranceHz float64 `json:"frequencyToleranceHz"`
	TrackerToleranceHz   float64 `json:"trackerToleranceHz"`
	TrackerTTLMS         int     `json:"trackerTtlMs"`
}

// LayoutSettings stores user-adjusted main-window region sizes. Heights are
// deliberately persisted so the operator's preferred working layout survives
// restarts; UI drag handlers enforce the min/max bounds at runtime.
type LayoutSettings struct {
	WaterfallHeight int `json:"waterfallHeight"`
	OperationHeight int `json:"operationHeight"`
	WindowWidth     int `json:"windowWidth"`
	WindowHeight    int `json:"windowHeight"`
}

type MacroConfig struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Shortcut string `json:"shortcut"`
	Template string `json:"template"`
	Enabled  bool   `json:"enabled"`
	Global   bool   `json:"global"`
}

type Settings struct {
	MyCall      string                  `json:"myCall"`
	MyGrid      string                  `json:"myGrid"`
	Audio       AudioSettings           `json:"audio"`
	Radio       RadioSettings           `json:"radio"`
	Waterfall   WaterfallSettings       `json:"waterfall"`
	Frequencies []FrequencyBandSettings `json:"frequencies"`
	Decoder     DecoderSettings         `json:"decoder"`
	Layout      LayoutSettings          `json:"layout"`
	Macros      []MacroConfig           `json:"macros"`
	Automation  []logbook.Rule          `json:"automation"`
	// LogbookPath is retained for backward compatibility; JTTY-Go always writes JTTY.adi in the data directory.
	LogbookPath          string `json:"logbookPath"`
	DecodeLogMode        string `json:"decodeLogMode"`
	DecodeLogEnabled     bool   `json:"decodeLogEnabled"`
	RecordEnabled        bool   `json:"recordEnabled"`
	DecodeWindowLimit    int    `json:"decodeWindowLimit"`
	AutoFollowTail       bool   `json:"autoFollowTail"`
	AutoStartMonitor     bool   `json:"autoStartMonitor"`
	ExchangeSerialNumber int    `json:"exchangeSerialNumber"`
	RXFrequencyHz        int    `json:"rxFrequencyHz"`
	TXFrequencyHz        int    `json:"txFrequencyHz"`
}

func DefaultFrequencyBands() []FrequencyBandSettings {
	defaults := []struct {
		band string
		hz   int64
	}{
		{"160m", 1838000},
		{"80m", 3575000},
		{"60m", 5357000},
		{"40m", 7090000},
		{"30m", 10140000},
		{"20m", 14090000},
		{"17m", 18100000},
		{"15m", 21090000},
		{"12m", 24920000},
		{"10m", 28090000},
		{"6m", 50316000},
		{"2m", 144077000},
		{"70cm", 432077000},
	}
	out := make([]FrequencyBandSettings, 0, len(defaults))
	for _, d := range defaults {
		out = append(out, FrequencyBandSettings{Band: d.band, Frequencies: []int64{d.hz}, DefaultHz: d.hz})
	}
	return out
}

func DefaultSettings() Settings {
	return Settings{
		Audio: AudioSettings{SampleRate: 48000, InputChannel: "Mono", OutputChannel: "Mono", BufferMS: 20, TxAudioLevel: 65},
		Radio: RadioSettings{
			Backend: "none", RigName: "None", RigModelID: 0, Host: "127.0.0.1", Port: 4532,
			Baud: 9600, DataBits: 8, StopBits: 1, Handshake: "None", PTTMethod: "VOX",
			SplitMode: "Rig", TXAudioSource: "Front", ForceDTR: "none", ForceRTS: "none", PollIntervalSec: 1, Mode: "PKTUSB", PassbandHz: 3000,
		},
		Waterfall:   WaterfallSettings{MinHz: 0, MaxHz: 2700, FFT: 4096, Hop: 1024},
		Frequencies: DefaultFrequencyBands(),
		Decoder:     DecoderSettings{Threads: 0, Mode: "realtime", ThresholdDb: 8, FrequencyToleranceHz: 20, TrackerToleranceHz: 20, TrackerTTLMS: 600},
		Layout:      LayoutSettings{WaterfallHeight: 210, OperationHeight: 270, WindowWidth: 1360, WindowHeight: 820},
		Macros: []MacroConfig{
			{ID: "F1", Name: "CQ", Shortcut: "F1", Template: "CQ %M CQ", Enabled: true, Global: true},
			{ID: "F2", Name: "Exchange", Shortcut: "F2", Template: "%H %E", Enabled: true, Global: true},
			{ID: "F3", Name: "TU + CQ", Shortcut: "F3", Template: "%H TU CQ %M CQ", Enabled: true, Global: true},
			{ID: "F4", Name: "My Call", Shortcut: "F4", Template: "%M", Enabled: true, Global: true},
			{ID: "F5", Name: "His Call", Shortcut: "F5", Template: "%H", Enabled: true, Global: true},
			{ID: "F6", Name: "TU Now", Shortcut: "F6", Template: "TU NOW %Q %E", Enabled: true, Global: true},
			{ID: "F7", Name: "Again", Shortcut: "F7", Template: "%H AGN?", Enabled: true, Global: true},
			{ID: "F8", Name: "Exchange", Shortcut: "F8", Template: "%E", Enabled: true, Global: true},
		},
		Automation:  nil,
		LogbookPath: "JTTY.adi", DecodeLogMode: "single", DecodeLogEnabled: true, RecordEnabled: false, DecodeWindowLimit: 2000, AutoFollowTail: true,
		AutoStartMonitor: false, ExchangeSerialNumber: 1, RXFrequencyHz: 1500, TXFrequencyHz: 1500,
	}
}
