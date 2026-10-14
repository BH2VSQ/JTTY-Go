package app

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/BH2VSQ/jtty-go/internal/model"
	"github.com/BH2VSQ/jtty-go/internal/radio"
)

// Like Wails, this transport delivers backend replies to Go listeners too.
// The old browser mock delivered replies only to JavaScript and hid the loop.
type echoTransport struct {
	mu        sync.Mutex
	handlers  map[string]func(...interface{})
	counts    map[string]int
	responses chan string
}

func (e *echoTransport) Emit(name string, payload any) {
	e.mu.Lock()
	e.counts[name]++
	handler := e.handlers[name]
	count := e.counts[name]
	e.mu.Unlock()
	if count > 20 {
		return
	} // Bound a regression so it fails instead of wedging tests.
	if handler != nil {
		handler(payload)
	}
	e.responses <- name
}
func (e *echoTransport) on(name string, callback func(...interface{})) { e.handlers[name] = callback }

func TestHamlibRequestResponseEventsDoNotSelfTrigger(t *testing.T) {
	a := New()
	a.hamlibModelsCached = true
	a.hamlibModelsCache = []radio.HamlibModel{{ModelID: 1, Label: "Hamlib Dummy"}}
	a.hamlibCapsCache[1] = radio.HamlibCapabilities{ModelID: 1, HasGetFreq: true}
	a.hamlibStatusCache = &radio.HamlibUpdateStatus{Architecture: "64"}
	bus := &echoTransport{handlers: map[string]func(...interface{}){}, counts: map[string]int{}, responses: make(chan string, 32)}
	a.SetEventSink(bus)
	a.registerHamlibEvents(bus.on)
	cases := []struct {
		request  string
		response model.EventName
		payload  any
	}{
		{"hamlib:models:request", model.EventHamlibModels, false},
		{"hamlib:capabilities:request", model.EventHamlibCapabilities, 1},
		{"hamlib:status:request", model.EventHamlibStatus, "64"},
		{"radio:serial-ports:request", model.EventRadioSerialPorts, ""},
	}
	for _, tc := range cases {
		if bus.handlers[string(tc.response)] != nil {
			t.Fatalf("reply %q re-enters a Go request handler", tc.response)
		}
		callback := bus.handlers[tc.request]
		if callback == nil {
			t.Fatal(tc.request)
		}
		for i := 0; i < 10; i++ {
			callback(tc.payload)
			select {
			case name := <-bus.responses:
				if name != string(tc.response) {
					t.Fatalf("reply=%s", name)
				}
			case <-time.After(time.Second):
				t.Fatal("request did not return")
			}
		}
		if bus.counts[string(tc.response)] != 10 {
			t.Fatalf("response storm: %+v", bus.counts)
		}
	}
}

func TestHamlibUncachedModelListReturnsOnce(t *testing.T) {
	executable, _ := filepath.Abs(filepath.Join("..", "..", "bin", "rigctld.exe"))
	if _, err := os.Stat(executable); err != nil {
		t.Skip("requires bundled Windows runtime")
	}
	a := New()
	a.settings.Radio.Local.RigctldPath = executable
	bus := &echoTransport{handlers: map[string]func(...interface{}){}, counts: map[string]int{}, responses: make(chan string, 32)}
	a.SetEventSink(bus)
	a.registerHamlibEvents(bus.on)
	bus.handlers["hamlib:models:request"](false)
	select {
	case name := <-bus.responses:
		if name != string(model.EventHamlibModels) {
			t.Fatal(name)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("model helper timed out")
	}
	a.hamlibCacheMu.RLock()
	models := len(a.hamlibModelsCache)
	a.hamlibCacheMu.RUnlock()
	if models < 100 {
		t.Fatalf("unexpected model count %d", models)
	}
	bus.handlers["hamlib:models:request"](false)
	select {
	case <-bus.responses:
	case <-time.After(time.Second):
		t.Fatal("cache did not reply")
	}
	bus.mu.Lock()
	count := bus.counts[string(model.EventHamlibModels)]
	bus.mu.Unlock()
	if count != 2 {
		t.Fatalf("cached response self-triggered %d times", count)
	}
}
