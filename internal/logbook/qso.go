package logbook

import "time"

type QSO struct {
	StartUTC        time.Time `json:"startUtc"`
	EndUTC          time.Time `json:"endUtc"`
	Call            string    `json:"call"`
	Grid            string    `json:"grid"`
	Name            string    `json:"name"`
	PowerWatts      float64   `json:"powerWatts"`
	Operator        string    `json:"operator"`
	StationCallsign string    `json:"stationCallsign"`
	StationGrid     string    `json:"stationGrid"`
	Band            string    `json:"band"`
	Frequency       int64     `json:"frequency"`
	Mode            string    `json:"mode"`
	RSTSent         string    `json:"rstSent"`
	RSTRcvd         string    `json:"rstRcvd"`
	ExchangeSent    string    `json:"exchangeSent"`
	ExchangeRecv    string    `json:"exchangeRecv"`
	StartMacro      string    `json:"startMacro"`
	EndMacro        string    `json:"endMacro"`
	FirstRXMessage  string    `json:"firstRxMessage"`
	LastRXMessage   string    `json:"lastRxMessage"`
}
