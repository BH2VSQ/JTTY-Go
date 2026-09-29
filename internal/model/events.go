package model

type EventName string

const (
	EventDecodeAdded            EventName = "decode:added"
	EventDecodeUpdated          EventName = "decode:updated"
	EventDecodeBatch            EventName = "decode:batch"
	EventDXCallChanged          EventName = "qso:dxcall"
	EventTxStarted              EventName = "tx:started"
	EventTxFinished             EventName = "tx:finished"
	EventHotkey                 EventName = "hotkey"
	EventWaterfall              EventName = "waterfall"
	EventCandidate              EventName = "detector:candidate"
	EventAudioDevices           EventName = "audio:devices"
	EventAudioState             EventName = "audio:state"
	EventAudioMeter             EventName = "audio:meter"
	EventQueueChanged           EventName = "tx:queue"
	EventAppReady               EventName = "app:ready"
	EventMacroTriggered         EventName = "macro:triggered"
	EventQSOLogged              EventName = "qso:logged"
	EventQSORecordResult        EventName = "qso:record-result"
	EventQSOStarted             EventName = "qso:started"
	EventRecordState            EventName = "record:state"
	EventRecordError            EventName = "record:error"
	EventLogError               EventName = "log:error"
	EventFrequencyRX            EventName = "frequency:rx"
	EventFrequencyTX            EventName = "frequency:tx"
	EventSettingsState          EventName = "settings:state"
	EventSettingsSaved          EventName = "settings:saved"
	EventSettingsError          EventName = "settings:error"
	EventRadioState             EventName = "radio:state"
	EventRadioTest              EventName = "radio:test-result"
	EventRadioMeterCapabilities EventName = "radio:meter-capabilities"
	EventRadioMeter             EventName = "radio:meter"
	EventHamlibStatus           EventName = "hamlib:status"
	EventHamlibModels           EventName = "hamlib:models"
	EventHamlibCapabilities     EventName = "hamlib:capabilities"
	EventRadioSerialPorts       EventName = "radio:serial-ports"
)

type DXCallChanged struct {
	Call string `json:"call"`
}
type HotkeyEvent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type MacroTriggered struct {
	ID      string `json:"id"`
	Message string `json:"message,omitempty"`
}
