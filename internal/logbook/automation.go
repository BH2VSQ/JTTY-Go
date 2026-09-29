package logbook

type TriggerEvent string

const (
	TxStarted  TriggerEvent = "tx_started"
	TxFinished TriggerEvent = "tx_finished"
)

type ActionType string

const (
	ActionQSOStart      ActionType = "qso_start"
	ActionCaptureDXCall ActionType = "capture_dx_call"
	ActionCreateQSO     ActionType = "create_qso"
)

type Trigger struct {
	Event TriggerEvent `json:"event"`
	Macro string       `json:"macro"`
}

type Action struct {
	Type ActionType `json:"type"`
}

type Rule struct {
	ID      string   `json:"id"`
	Enabled bool     `json:"enabled"`
	Trigger Trigger  `json:"trigger"`
	Actions []Action `json:"actions"`
}
