package radio

// PTTSerial controls hardware DTR/RTS lines when CAT PTT is not desired.
type PTTSerial interface {
	Set(port, method string, on bool) error
	Close() error
}
