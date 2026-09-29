package logbook

import (
	"testing"
	"time"
)

func TestEngineRules(t *testing.T) {
	e := NewEngine([]Rule{{ID: "start", Enabled: true, Trigger: Trigger{Event: TxFinished, Macro: "F1"}, Actions: []Action{{Type: ActionQSOStart}}}, {ID: "finish", Enabled: true, Trigger: Trigger{Event: TxFinished, Macro: "F3"}, Actions: []Action{{Type: ActionCaptureDXCall}, {Type: ActionCreateQSO}}}})
	t1 := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	r := e.TxFinished(TXContext{MacroID: "F1", DXCall: "JA1ABC", NowUTC: t1})
	if !r.QSOStarted {
		t.Fatal("qso not started")
	}
	r = e.TxFinished(TXContext{MacroID: "F3", DXCall: "JA1ABC", Message: "JA1ABC 599 001", NowUTC: t1.Add(time.Minute)})
	if r.QSO == nil || r.QSO.Call != "JA1ABC" {
		t.Fatalf("unexpected %+v", r.QSO)
	}
}
