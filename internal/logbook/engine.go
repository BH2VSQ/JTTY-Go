package logbook

import "time"

type Engine struct {
	rules   []Rule
	current *QSO
}

type TXContext struct {
	MacroID      string
	Message      string
	NowUTC       time.Time
	DXCall       string
	Band         string
	Frequency    int64
	Mode         string
	ExchangeSent string
	ExchangeRecv string
}

type Result struct {
	QSOStarted   bool
	CapturedCall string
	QSO          *QSO
}

func NewEngine(rules []Rule) *Engine    { return &Engine{rules: append([]Rule(nil), rules...)} }
func (e *Engine) SetRules(rules []Rule) { e.rules = append([]Rule(nil), rules...) }

func (e *Engine) TxFinished(ctx TXContext) Result {
	if ctx.NowUTC.IsZero() {
		ctx.NowUTC = time.Now().UTC()
	}
	result := Result{}
	dx := ctx.DXCall
	for _, r := range e.rules {
		if !r.Enabled || r.Trigger.Event != TxFinished || r.Trigger.Macro != ctx.MacroID {
			continue
		}
		for _, action := range r.Actions {
			switch action.Type {
			case ActionQSOStart:
				if e.current == nil {
					e.current = &QSO{StartUTC: ctx.NowUTC, Call: dx, Band: ctx.Band, Frequency: ctx.Frequency, Mode: ctx.Mode, StartMacro: ctx.MacroID}
					result.QSOStarted = true
				}
			case ActionCaptureDXCall:
				if dx != "" {
					result.CapturedCall = dx
					if e.current != nil && e.current.Call == "" {
						e.current.Call = dx
					}
				}
			case ActionCreateQSO:
				if e.current == nil && dx != "" {
					e.current = &QSO{StartUTC: ctx.NowUTC, Call: dx, Band: ctx.Band, Frequency: ctx.Frequency, Mode: ctx.Mode, StartMacro: ctx.MacroID}
				}
				if e.current != nil && e.current.Call == "" {
					e.current.Call = dx
				}
				if e.current != nil {
					e.current.EndUTC = ctx.NowUTC
					e.current.EndMacro = ctx.MacroID
					e.current.ExchangeSent = ctx.ExchangeSent
					e.current.ExchangeRecv = ctx.ExchangeRecv
					e.current.LastRXMessage = ctx.Message
					copyQSO := *e.current
					result.QSO = &copyQSO
					e.current = nil
				}
			}
		}
	}
	if result.QSO == nil && e.current != nil && e.current.Call == "" && dx != "" {
		e.current.Call = dx
	}
	return result
}
