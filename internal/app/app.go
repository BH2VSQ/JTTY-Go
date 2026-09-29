package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BH2VSQ/jtty-go/internal/audio"
	"github.com/BH2VSQ/jtty-go/internal/decode"
	"github.com/BH2VSQ/jtty-go/internal/detector"
	"github.com/BH2VSQ/jtty-go/internal/eventbus"
	"github.com/BH2VSQ/jtty-go/internal/hotkey"
	"github.com/BH2VSQ/jtty-go/internal/jtty"
	"github.com/BH2VSQ/jtty-go/internal/logbook"
	"github.com/BH2VSQ/jtty-go/internal/macro"
	"github.com/BH2VSQ/jtty-go/internal/model"
	"github.com/BH2VSQ/jtty-go/internal/radio"
	"github.com/BH2VSQ/jtty-go/internal/receiver"
	"github.com/BH2VSQ/jtty-go/internal/storage"
	"github.com/BH2VSQ/jtty-go/internal/tx"
	"github.com/BH2VSQ/jtty-go/internal/waterfall"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const minMainWindowWidth = 1080

type EventSink interface {
	Emit(event string, payload any)
}

type captureFrameSink struct{ app *App }

func (s captureFrameSink) OnPCMFrame(frame audio.PCMFrame) {
	if s.app != nil {
		s.app.OnPCMFrame(frame)
	}
}

type App struct {
	ctx                  context.Context
	embeddedAssets       fs.FS
	sink                 EventSink
	bus                  *eventbus.Bus
	settingsMu           sync.RWMutex
	frequencyMu          sync.RWMutex
	radioMu              sync.Mutex
	meterMu              sync.Mutex
	settings             model.Settings
	rxFrequency          int
	txFrequency          int
	configPath           string
	dxCall               string
	dxGrid               string
	dialFrequency        int64
	sequence             atomic.Uint64
	store                *decode.Store
	audioManager         audio.Manager
	capture              audio.Source
	pipeline             *receiver.Pipeline
	hotkeys              hotkey.Manager
	radio                radio.Radio
	rigctldCmd           *exec.Cmd
	radioTestMu          sync.Mutex
	testRadio            radio.Radio
	testRigctldCmd       *exec.Cmd
	testRadioCfg         model.RadioSettings
	testRadioOnline      bool
	testRestoreLiveRadio bool
	hamlibCacheMu        sync.RWMutex
	hamlibModelsCache    []radio.HamlibModel
	hamlibModelsCached   bool
	hamlibCapsCache      map[int]radio.HamlibCapabilities
	hamlibStatusCache    *radio.HamlibUpdateStatus
	hamlibOpsMu          sync.Mutex
	hamlibOps            map[uint64]context.CancelFunc
	hamlibOpSeq          uint64
	hamlibExecMu         sync.Mutex
	txQueue              *tx.Queue
	decodeLogger         *logbook.DecodeLogger
	recorder             *audio.Recorder
	qsoMu                sync.Mutex
	qsoStarts            map[string]time.Time
	jttyStream           *receiver.JTTYStream
	audioOutput          audio.Output
	pttSerial            radio.PTTSerial
	pollCancel           context.CancelFunc
	pollWG               sync.WaitGroup
	txMu                 sync.Mutex
	txCancel             context.CancelFunc
	txKind               string
	macroMu              sync.Mutex
	lastMacroID          string
	lastMacroTrigger     time.Time
	txID                 uint64
	receiverMu           sync.Mutex
	registeredHotkeys    map[string]struct{}
	startupOnce          sync.Once
	bandMu               sync.RWMutex
	currentBand          string
	duplexMode           atomic.Bool
	meterCancel          context.CancelFunc
	meterWG              sync.WaitGroup
	lastAudioMeter       time.Time
	lastWaterfallEmitNS  atomic.Int64
	lastCandidateEmitNS  atomic.Int64
	rxPaused             atomic.Bool
	txWaveScratch        []float32
	txDphiScratch        []float64
	txPulseScratch       []float64
}

func New() *App {
	settings := model.DefaultSettings()
	a := &App{bus: eventbus.New(), settings: settings, store: decode.NewStore(settings.DecodeWindowLimit), audioManager: audio.NewManager(), hotkeys: hotkey.NewManager(), txQueue: tx.NewQueue(), decodeLogger: logbook.NewDecodeLogger(), recorder: audio.NewRecorder(), qsoStarts: make(map[string]time.Time), rxFrequency: 1500, txFrequency: 1500, dialFrequency: 14090000, pttSerial: radio.NewPTTSerial(), registeredHotkeys: make(map[string]struct{}), currentBand: "20", hamlibCapsCache: make(map[int]radio.HamlibCapabilities), hamlibOps: make(map[uint64]context.CancelFunc)}
	a.hotkeys.SetHandler(func(b hotkey.Binding) {
		a.emit(model.EventHotkey, model.HotkeyEvent{ID: b.ID, Name: b.Name})
		a.triggerAndTransmitMacro(b.ID)
	})
	return a
}

// SetEmbeddedAssets provides the Wails application with embedded resources
// such as the bundled Hamlib runtime files.
func (a *App) SetEmbeddedAssets(assets fs.FS) { a.embeddedAssets = assets }

func (a *App) SetEventSink(s EventSink) { a.sink = s }
func (a *App) emit(event model.EventName, payload any) {
	if a.sink != nil {
		a.sink.Emit(string(event), payload)
	}
	a.bus.Publish(event, payload)
}

// beginHamlibOperation creates a cancellable, time-bounded context for a
// Hamlib helper process. Settings-page probes must never survive indefinitely
// after the page is closed. All helper processes are also serialized by
// hamlibExecMu in their event handlers to avoid launching a burst of
// rigctl/rigctld processes at once.
func (a *App) beginHamlibOperation(timeout time.Duration) (context.Context, func()) {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	a.hamlibOpsMu.Lock()
	a.hamlibOpSeq++
	id := a.hamlibOpSeq
	a.hamlibOps[id] = cancel
	a.hamlibOpsMu.Unlock()

	done := func() {
		a.hamlibOpsMu.Lock()
		delete(a.hamlibOps, id)
		a.hamlibOpsMu.Unlock()
		cancel()
	}
	return ctx, done
}

func (a *App) cancelHamlibOperations() {
	a.hamlibOpsMu.Lock()
	ops := make([]context.CancelFunc, 0, len(a.hamlibOps))
	for id, cancel := range a.hamlibOps {
		delete(a.hamlibOps, id)
		ops = append(ops, cancel)
	}
	a.hamlibOpsMu.Unlock()
	for _, cancel := range ops {
		cancel()
	}
}

func cloneHamlibModels(models []radio.HamlibModel) []radio.HamlibModel {
	return append([]radio.HamlibModel(nil), models...)
}

func (a *App) invalidateHamlibCaches() {
	a.hamlibCacheMu.Lock()
	a.hamlibModelsCache = nil
	a.hamlibModelsCached = false
	a.hamlibCapsCache = make(map[int]radio.HamlibCapabilities)
	a.hamlibStatusCache = nil
	a.hamlibCacheMu.Unlock()
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	wailsruntime.EventsOn(ctx, "duplex:set", func(data ...interface{}) {
		enabled := false
		if len(data) > 0 {
			if v, ok := data[0].(bool); ok {
				enabled = v
			}
		}
		a.duplexMode.Store(enabled)
		a.emitRaw("duplex:state", map[string]any{"enabled": enabled})
	})
	wailsruntime.EventsOn(ctx, "window:size-save", func(data ...interface{}) {
		_ = data
		if a.ctx == nil {
			return
		}
		w, h := wailsruntime.WindowGetSize(a.ctx)
		if w < minMainWindowWidth {
			w = minMainWindowWidth
		}
		if h < 680 {
			h = 680
		}
		a.settingsMu.Lock()
		a.settings.Layout.WindowWidth = w
		a.settings.Layout.WindowHeight = h
		settings := a.settings
		a.settingsMu.Unlock()
		if a.configPath != "" {
			if err := storage.SaveSettings(a.configPath, settings); err != nil {
				a.emit(model.EventSettingsError, map[string]any{"error": err.Error()})
			}
		}
	})
	wailsruntime.EventsOn(ctx, "frequency:set-rx", func(data ...interface{}) {
		if len(data) == 0 {
			return
		}
		if hz, ok := numericFrequency(data[0]); ok {
			_ = a.SetRXFrequency(hz)
		}
	})
	wailsruntime.EventsOn(ctx, "frequency:set-tx", func(data ...interface{}) {
		if len(data) == 0 {
			return
		}
		if hz, ok := numericFrequency(data[0]); ok {
			_ = a.SetTXFrequency(hz)
		}
	})
	wailsruntime.EventsOn(ctx, "frequency-range:set-max", func(data ...interface{}) {
		if len(data) == 0 {
			return
		}
		maxHz, ok := numericFrequency(data[0])
		if !ok {
			return
		}
		allowed := map[int]bool{2500: true, 2700: true, 3000: true, 3500: true, 4000: true}
		if !allowed[maxHz] {
			return
		}
		a.settingsMu.Lock()
		a.settings.Waterfall.MinHz = 0
		a.settings.Waterfall.MaxHz = float64(maxHz)
		settings := a.settings
		a.settingsMu.Unlock()
		if a.configPath != "" {
			if err := storage.SaveSettings(a.configPath, settings); err != nil {
				a.emit(model.EventSettingsError, map[string]any{"error": err.Error()})
				return
			}
		}
		a.receiverMu.Lock()
		pipe := a.pipeline
		a.receiverMu.Unlock()
		if pipe != nil {
			pipe.SetFrequencyRange(0, float64(maxHz))
		}
		a.emit(model.EventSettingsState, settings)
		a.emit(model.EventSettingsSaved, map[string]any{"ok": true, "frequencyMaxHz": maxHz})
	})
	wailsruntime.EventsOn(ctx, "jtty:set-tolerance", func(data ...interface{}) {
		if len(data) == 0 {
			return
		}
		n, ok := numericFrequency(data[0])
		if !ok {
			return
		}
		if n < 1 {
			n = 1
		}
		if n > 500 {
			n = 500
		}
		a.settingsMu.Lock()
		a.settings.Decoder.FrequencyToleranceHz = float64(n)
		a.settingsMu.Unlock()
		if a.jttyStream != nil {
			a.jttyStream.SetFrequencyTolerance(float64(n))
		}
		a.emit(model.EventSettingsState, a.Settings())
	})
	wailsruntime.EventsOn(ctx, "audio:set-tx-level", func(data ...interface{}) {
		if len(data) == 0 {
			return
		}
		n, ok := numericFrequency(data[0])
		if !ok {
			return
		}
		if n < 0 {
			n = 0
		}
		if n > 100 {
			n = 100
		}
		a.settingsMu.Lock()
		a.settings.Audio.TxAudioLevel = n
		settings := a.settings
		a.settingsMu.Unlock()
		if a.configPath != "" {
			_ = storage.SaveSettings(a.configPath, settings)
		}
		a.emit(model.EventSettingsState, settings)
	})
	wailsruntime.EventsOn(ctx, "frequency:set-dial", func(data ...interface{}) {
		if len(data) == 0 {
			return
		}
		if hz, ok := numericFrequency64(data[0]); ok {
			if err := a.SetDialFrequency(hz); err != nil {
				a.emitRaw("frequency:error", map[string]any{"error": err.Error()})
			}
		}
	})
	wailsruntime.EventsOn(ctx, "settings:get", func(data ...interface{}) {
		_ = data
		a.emit(model.EventSettingsState, a.Settings())
	})
	wailsruntime.EventsOn(ctx, "settings:save", func(data ...interface{}) {
		if len(data) == 0 {
			a.emit(model.EventSettingsError, map[string]any{"error": "missing settings payload"})
			return
		}
		b, err := json.Marshal(data[0])
		if err != nil {
			a.emit(model.EventSettingsError, map[string]any{"error": err.Error()})
			return
		}
		var settings model.Settings
		if err := json.Unmarshal(b, &settings); err != nil {
			a.emit(model.EventSettingsError, map[string]any{"error": err.Error()})
			return
		}
		if err := a.SaveSettings(settings); err != nil {
			a.emit(model.EventSettingsError, map[string]any{"error": err.Error()})
			return
		}
		a.emit(model.EventSettingsSaved, map[string]any{"ok": true})
	})
	home, err := os.UserConfigDir()
	if err == nil {
		a.configPath = filepath.Join(home, "JTTY-Go", "config.json")
		if settings, loadErr := storage.LoadSettings(a.configPath); loadErr == nil {
			settings, migrated := normalizeLoadedSettings(settings, filepath.Join(home, "JTTY-Go", "record"))
			a.settingsMu.Lock()
			a.settings = settings
			a.store = decode.NewStore(settings.DecodeWindowLimit)
			a.settingsMu.Unlock()
			a.frequencyMu.Lock()
			a.rxFrequency = settings.RXFrequencyHz
			a.txFrequency = settings.TXFrequencyHz
			a.frequencyMu.Unlock()
			if ctx != nil && settings.Layout.WindowWidth > 0 && settings.Layout.WindowHeight > 0 {
				wailsruntime.WindowSetSize(ctx, settings.Layout.WindowWidth, settings.Layout.WindowHeight)
			}
			if migrated {
				_ = storage.SaveSettings(a.configPath, settings)
			}
		}
	}
	if err := a.ensureBundledHamlib(); err != nil {
		a.emitRaw("hamlib:bundle-error", map[string]any{"error": err.Error()})
	}
	wailsruntime.EventsOn(ctx, "qso:dxcall", func(data ...interface{}) {
		if len(data) == 0 {
			return
		}
		if v, ok := data[0].(string); ok {
			a.SetDXCall(v)
		}
	})
	wailsruntime.EventsOn(ctx, "qso:serial-set", func(data ...interface{}) {
		if len(data) == 0 {
			return
		}
		n, ok := numericFrequency(data[0])
		if !ok {
			return
		}
		serial := int(math.Round(float64(n)))
		if serial < 1 {
			serial = 1
		}
		a.settingsMu.Lock()
		settings := a.settings
		settings.ExchangeSerialNumber = serial
		a.settingsMu.Unlock()
		if err := a.SaveSettings(settings); err != nil {
			a.emit(model.EventSettingsError, map[string]any{"error": err.Error()})
			return
		}
	})
	wailsruntime.EventsOn(ctx, "qso:dxgrid", func(data ...interface{}) {
		if len(data) == 0 {
			return
		}
		if v, ok := data[0].(string); ok {
			a.SetDXGrid(v)
		}
	})
	wailsruntime.EventsOn(ctx, "tx:send", func(data ...interface{}) {
		message := ""
		call := ""
		if len(data) > 0 {
			switch v := data[0].(type) {
			case string:
				message = v
			case map[string]interface{}:
				if x, ok := v["message"].(string); ok {
					message = x
				}
				if x, ok := v["call"].(string); ok {
					call = x
				}
			}
		}
		if strings.TrimSpace(call) != "" {
			a.SetDXCall(call)
		}
		if strings.TrimSpace(message) != "" {
			go a.transmitJTTY("manual", message)
		}
	})
	wailsruntime.EventsOn(ctx, "macro:trigger", func(data ...interface{}) {
		if len(data) == 0 {
			return
		}
		id, _ := data[0].(string)
		a.triggerAndTransmitMacro(id)
	})
	wailsruntime.EventsOn(ctx, "tx:tune", func(data ...interface{}) {
		_ = data
		a.txMu.Lock()
		tuning := a.txCancel != nil && a.txKind == "tune"
		busy := a.txCancel != nil && a.txKind != "tune"
		a.txMu.Unlock()
		if tuning {
			a.StopTransmit()
			return
		}
		if busy {
			a.emitRaw("tx:error", map[string]any{"error": "已有发射正在进行"})
			return
		}
		go a.tuneJTTY(1500)
	})
	wailsruntime.EventsOn(ctx, "tx:stop", func(data ...interface{}) { _ = data; a.StopTransmit() })
	wailsruntime.EventsOn(ctx, "band:set", func(data ...interface{}) {
		if len(data) == 0 {
			return
		}
		v, ok := data[0].(string)
		band := strings.TrimSpace(v)
		if !ok || band == "" {
			return
		}
		freq, exists := bandDialFrequencyFromSettings(a.Settings(), band)
		if !exists {
			a.emitRaw("frequency:error", map[string]any{"error": "未定义的波段频率"})
			return
		}
		if err := a.SetDialFrequency(freq); err != nil {
			a.emitRaw("frequency:error", map[string]any{"error": err.Error()})
			return
		}
		a.bandMu.Lock()
		a.currentBand = band
		a.bandMu.Unlock()
		a.emitRaw("band:state", map[string]any{"band": band, "frequencyHz": freq})
	})
	wailsruntime.EventsOn(ctx, "receiver:decode-now", func(data ...interface{}) {
		_ = data
		a.receiverMu.Lock()
		stream := a.jttyStream
		a.receiverMu.Unlock()
		ok := false
		if stream != nil {
			ok = stream.DecodeNow()
		}
		a.emitRaw("decode:manual-result", map[string]any{"ok": ok})
	})
	wailsruntime.EventsOn(ctx, "receiver:start", func(data ...interface{}) {
		_ = data
		go func() {
			if err := a.StartReceiver(); err != nil {
				a.emit(model.EventAudioState, map[string]any{"running": false, "error": err.Error()})
			}
		}()
	})
	wailsruntime.EventsOn(ctx, "receiver:stop", func(data ...interface{}) {
		_ = data
		go func() { _ = a.StopReceiver() }()
	})
	wailsruntime.EventsOn(ctx, "qso:record", func(data ...interface{}) {
		var payload any
		if len(data) > 0 {
			payload = data[0]
		}
		if err := a.recordQSOFromPayload(payload); err != nil {
			a.emit(model.EventQSORecordResult, map[string]any{"ok": false, "error": err.Error()})
		}
	})
	wailsruntime.EventsOn(ctx, "save:set", func(data ...interface{}) {
		var payload any
		if len(data) > 0 {
			payload = data[0]
		}
		if err := a.updateSaveOptions(payload); err != nil {
			a.emit(model.EventSettingsError, map[string]any{"error": err.Error()})
		}
	})

	wailsruntime.EventsOn(ctx, "audio:refresh", func(data ...interface{}) {
		_ = data
		inputs, inErr := a.audioManager.Inputs()
		outputs, outErr := a.audioManager.Outputs()
		a.emit(model.EventAudioDevices, map[string]any{"inputs": inputs, "outputs": outputs, "inputError": errString(inErr), "outputError": errString(outErr)})
	})
	wailsruntime.EventsOn(ctx, "radio:connect", func(data ...interface{}) {
		_ = data
		if err := a.ConnectRadio(); err != nil {
			a.emit(model.EventRadioTest, map[string]any{"ok": false, "kind": "connect", "error": err.Error()})
			return
		}
	})
	wailsruntime.EventsOn(ctx, "radio:disconnect", func(data ...interface{}) {
		_ = data
		_ = a.DisconnectRadio()
	})
	wailsruntime.EventsOn(ctx, "radio:test-cat", func(data ...interface{}) {
		result := a.TestRadio()
		if len(data) > 0 {
			b, err := json.Marshal(data[0])
			if err == nil {
				var draft model.Settings
				if err = json.Unmarshal(b, &draft); err == nil {
					result = a.TestRadioSettings(draft)
				}
			}
		}
		result["kind"] = "cat"
		a.emit(model.EventRadioTest, result)
	})
	wailsruntime.EventsOn(ctx, "radio:test-ptt", func(data ...interface{}) {
		on := false
		cfg := a.Settings().Radio
		if len(data) > 0 {
			switch v := data[0].(type) {
			case bool:
				on = v
			case map[string]interface{}:
				if x, ok := v["on"].(bool); ok {
					on = x
				}
				if raw, ok := v["settings"]; ok {
					b, err := json.Marshal(raw)
					if err == nil {
						var draft model.Settings
						if json.Unmarshal(b, &draft) == nil {
							cfg = draft.Radio
						}
					}
				}
			}
		}
		result := a.testPTTConfig(cfg, on)
		result["kind"] = "ptt"
		a.emit(model.EventRadioTest, result)
	})
	wailsruntime.EventsOn(ctx, "radio:test-end", func(data ...interface{}) {
		a.cancelHamlibOperations()
		restore := true
		if len(data) > 0 {
			if raw, ok := data[0].(map[string]interface{}); ok {
				if v, exists := raw["restore"].(bool); exists {
					restore = v
				}
			}
		}
		a.closeTestRadio(restore)
	})
	wailsruntime.EventsOn(ctx, "hamlib:cancel", func(data ...interface{}) {
		_ = data
		a.cancelHamlibOperations()
	})
	wailsruntime.EventsOn(ctx, "hamlib:models", func(data ...interface{}) {
		force := false
		if len(data) > 0 {
			switch v := data[0].(type) {
			case string:
				force = strings.EqualFold(strings.TrimSpace(v), "force")
			case map[string]interface{}:
				if raw, ok := v["force"].(bool); ok {
					force = raw
				}
			}
		}

		a.hamlibCacheMu.RLock()
		cached := a.hamlibModelsCached && !force
		models := cloneHamlibModels(a.hamlibModelsCache)
		a.hamlibCacheMu.RUnlock()
		if cached {
			a.emit(model.EventHamlibModels, models)
			return
		}

		ctxProbe, done := a.beginHamlibOperation(10 * time.Second)
		defer done()
		a.hamlibExecMu.Lock()
		defer a.hamlibExecMu.Unlock()

		// Re-check the cache after taking the serialization lock. Another UI
		// event may have populated it while this request was waiting.
		a.hamlibCacheMu.RLock()
		cached = a.hamlibModelsCached && !force
		models = cloneHamlibModels(a.hamlibModelsCache)
		a.hamlibCacheMu.RUnlock()
		if cached {
			a.emit(model.EventHamlibModels, models)
			return
		}

		models, err := radio.ListHamlibModels(ctxProbe, a.hamlibExecutable())
		if err != nil {
			if ctxProbe.Err() != nil {
				return
			}
			a.emit(model.EventSettingsError, map[string]any{"error": err.Error()})
			return
		}
		a.hamlibCacheMu.Lock()
		a.hamlibModelsCache = cloneHamlibModels(models)
		a.hamlibModelsCached = true
		a.hamlibCacheMu.Unlock()
		a.emit(model.EventHamlibModels, models)
	})
	wailsruntime.EventsOn(ctx, "hamlib:capabilities", func(data ...interface{}) {
		modelID := 0
		if len(data) > 0 {
			switch v := data[0].(type) {
			case int:
				modelID = v
			case float64:
				modelID = int(v)
			case string:
				modelID, _ = strconv.Atoi(strings.TrimSpace(v))
			case map[string]interface{}:
				if raw, ok := v["modelId"]; ok {
					switch n := raw.(type) {
					case float64:
						modelID = int(n)
					case int:
						modelID = n
					}
				}
			}
		}
		if modelID <= 0 {
			return
		}

		a.hamlibCacheMu.RLock()
		cachedCaps, cachedOK := a.hamlibCapsCache[modelID]
		a.hamlibCacheMu.RUnlock()
		if cachedOK {
			a.emit(model.EventHamlibCapabilities, map[string]any{"modelId": modelID, "available": true, "capabilities": cachedCaps})
			return
		}

		ctxProbe, done := a.beginHamlibOperation(10 * time.Second)
		defer done()
		a.hamlibExecMu.Lock()
		defer a.hamlibExecMu.Unlock()

		// A duplicate capability request can arrive while another request was
		// waiting on the serialization lock. Check the cache again before
		// starting another rigctl process.
		a.hamlibCacheMu.RLock()
		cachedCaps, cachedOK = a.hamlibCapsCache[modelID]
		a.hamlibCacheMu.RUnlock()
		if cachedOK {
			a.emit(model.EventHamlibCapabilities, map[string]any{"modelId": modelID, "available": true, "capabilities": cachedCaps})
			return
		}

		caps, err := radio.HamlibModelCapabilities(ctxProbe, a.hamlibExecutable(), modelID)
		if err != nil {
			if ctxProbe.Err() != nil {
				return
			}
			a.emit(model.EventHamlibCapabilities, map[string]any{"modelId": modelID, "error": err.Error(), "available": false})
			return
		}
		a.hamlibCacheMu.Lock()
		if a.hamlibCapsCache == nil {
			a.hamlibCapsCache = make(map[int]radio.HamlibCapabilities)
		}
		a.hamlibCapsCache[modelID] = caps
		a.hamlibCacheMu.Unlock()
		a.emit(model.EventHamlibCapabilities, map[string]any{"modelId": modelID, "available": true, "capabilities": caps})
	})
	wailsruntime.EventsOn(ctx, "radio:serial-ports", func(data ...interface{}) {
		_ = data
		a.emit(model.EventRadioSerialPorts, radio.AvailableSerialPorts())
	})
	wailsruntime.EventsOn(ctx, "hamlib:status", func(data ...interface{}) {
		arch := "64"
		if len(data) > 0 {
			if v, ok := data[0].(string); ok && v != "" {
				arch = v
			}
		}

		a.hamlibCacheMu.RLock()
		statusCached := a.hamlibStatusCache != nil && a.hamlibStatusCache.Architecture == arch
		var cachedStatus radio.HamlibUpdateStatus
		if statusCached {
			cachedStatus = *a.hamlibStatusCache
		}
		a.hamlibCacheMu.RUnlock()
		if statusCached {
			a.emit(model.EventHamlibStatus, cachedStatus)
			return
		}

		ctxProbe, done := a.beginHamlibOperation(10 * time.Second)
		defer done()
		a.hamlibExecMu.Lock()
		defer a.hamlibExecMu.Unlock()

		a.hamlibCacheMu.RLock()
		statusCached = a.hamlibStatusCache != nil && a.hamlibStatusCache.Architecture == arch
		if statusCached {
			cachedStatus = *a.hamlibStatusCache
		}
		a.hamlibCacheMu.RUnlock()
		if statusCached {
			a.emit(model.EventHamlibStatus, cachedStatus)
			return
		}

		// The status probe is cancellable and uses the same hidden-process path as
		// the model/capability probes, so closing Settings can stop it cleanly.
		status, err := radio.HamlibStatusWithExecutableContext(ctxProbe, arch, a.hamlibInstallDir(), a.hamlibExecutable())
		if err != nil {
			if ctxProbe.Err() != nil {
				return
			}
			a.emit(model.EventSettingsError, map[string]any{"error": err.Error()})
			return
		}
		a.hamlibCacheMu.Lock()
		statusCopy := status
		a.hamlibStatusCache = &statusCopy
		a.hamlibCacheMu.Unlock()
		a.emit(model.EventHamlibStatus, status)
	})
	wailsruntime.EventsOn(ctx, "hamlib:update", func(data ...interface{}) {
		a.invalidateHamlibCaches()
		arch := "64"
		if len(data) > 0 {
			if v, ok := data[0].(string); ok && v != "" {
				arch = v
			}
		}
		_ = a.DisconnectRadio()
		status, err := radio.UpdateHamlib(context.Background(), arch, a.hamlibInstallDir())
		if err != nil {
			a.emit(model.EventSettingsError, map[string]any{"error": err.Error()})
			return
		}
		if verified, probeErr := radio.HamlibStatusWithExecutable(arch, a.hamlibInstallDir(), a.hamlibExecutable()); probeErr == nil {
			status = verified
		}
		a.hamlibCacheMu.Lock()
		statusCopy := status
		a.hamlibStatusCache = &statusCopy
		a.hamlibCacheMu.Unlock()
		a.emit(model.EventHamlibStatus, status)
		if models, modelErr := radio.ListHamlibModels(context.Background(), a.hamlibExecutable()); modelErr == nil {
			a.hamlibCacheMu.Lock()
			a.hamlibModelsCache = cloneHamlibModels(models)
			a.hamlibModelsCached = true
			a.hamlibCacheMu.Unlock()
			a.emit(model.EventHamlibModels, models)
		}
	})
	wailsruntime.EventsOn(ctx, "file:open-adi", func(data ...interface{}) {
		_ = data
		err := a.OpenADIFLog()
		a.emitRaw("file:result", map[string]any{"action": "open-adi", "ok": err == nil, "error": errString(err), "path": filepath.Join(a.dataDir(), "JTTY.adi")})
	})
	wailsruntime.EventsOn(ctx, "file:delete-all", func(data ...interface{}) {
		_ = data
		err := a.DeleteALLLogs()
		a.emitRaw("file:result", map[string]any{"action": "delete-all", "ok": err == nil, "error": errString(err)})
	})
	wailsruntime.EventsOn(ctx, "file:delete-adi", func(data ...interface{}) {
		_ = data
		err := a.DeleteADIF()
		a.emitRaw("file:result", map[string]any{"action": "delete-adi", "ok": err == nil, "error": errString(err)})
	})
	wailsruntime.EventsOn(ctx, "file:open-data-dir", func(data ...interface{}) {
		_ = data
		err := a.OpenDataDirectory()
		a.emitRaw("file:result", map[string]any{"action": "open-data-dir", "ok": err == nil, "error": errString(err), "path": a.dataDir()})
	})
	wailsruntime.EventsOn(ctx, "app:quit", func(data ...interface{}) {
		_ = data
		if a.ctx != nil {
			wailsruntime.Quit(a.ctx)
		}
	})

	wailsruntime.EventsOn(ctx, "hamlib:revert", func(data ...interface{}) {
		a.invalidateHamlibCaches()
		arch := "64"
		if len(data) > 0 {
			if v, ok := data[0].(string); ok && v != "" {
				arch = v
			}
		}
		_ = a.DisconnectRadio()
		status, err := radio.RevertHamlib(arch, a.hamlibInstallDir())
		if err != nil {
			a.emit(model.EventSettingsError, map[string]any{"error": err.Error()})
			return
		}
		if verified, probeErr := radio.HamlibStatusWithExecutable(arch, a.hamlibInstallDir(), a.hamlibExecutable()); probeErr == nil {
			status = verified
		}
		a.hamlibCacheMu.Lock()
		statusCopy := status
		a.hamlibStatusCache = &statusCopy
		a.hamlibCacheMu.Unlock()
		a.emit(model.EventHamlibStatus, status)
	})
	a.emit(model.EventSettingsState, a.Settings())
	a.frequencyMu.RLock()
	rxHz := a.rxFrequency
	txHz := a.txFrequency
	a.frequencyMu.RUnlock()
	a.emit(model.EventFrequencyRX, map[string]any{"frequencyHz": rxHz})
	a.emit(model.EventFrequencyTX, map[string]any{"frequencyHz": txHz})
	a.emitRaw("duplex:state", map[string]any{"enabled": a.duplexMode.Load()})
	a.emitRaw("qso:dxgrid", map[string]any{"grid": a.DXGrid()})
	a.emitRecordState()
}

func normalizeLoadedSettings(s model.Settings, recordDir string) (model.Settings, bool) {
	changed := false
	// The radio model is the authoritative device selection. Backend remains
	// persisted only for backward compatibility with older settings files.
	if s.Radio.RigModelID <= 0 {
		if s.Radio.Backend != "none" || s.Radio.RigName != "None" {
			changed = true
		}
		s.Radio.Backend = "none"
		s.Radio.RigModelID = 0
		s.Radio.RigName = "None"
		if !strings.EqualFold(strings.TrimSpace(s.Radio.PTTMethod), "VOX") {
			s.Radio.PTTMethod = "VOX"
			changed = true
		}
	} else {
		if s.Radio.Backend != "hamlib-rigctld" {
			s.Radio.Backend = "hamlib-rigctld"
			changed = true
		}
		if strings.TrimSpace(s.Radio.RigName) == "" || strings.EqualFold(strings.TrimSpace(s.Radio.RigName), "None") {
			// Keep the numeric model selection even if the human label is not
			// available until Hamlib is enumerated again. Do not mark the settings
			// dirty here: enumeration will supply the display name when available.
		}
		method := strings.ToUpper(strings.TrimSpace(s.Radio.PTTMethod))
		if method != "VOX" && method != "CAT" && method != "DTR" && method != "RTS" {
			s.Radio.PTTMethod = "VOX"
			changed = true
		}
	}
	if strings.TrimSpace(s.Audio.RecordDirectory) == "" {
		s.Audio.RecordDirectory = recordDir
		changed = true
	}
	if s.Waterfall.MinHz != 0 {
		s.Waterfall.MinHz = 0
		changed = true
	}
	safeMax := normalizeFrequencyMax(s.Waterfall.MaxHz)
	if s.Waterfall.MaxHz != safeMax {
		s.Waterfall.MaxHz = safeMax
		changed = true
	}
	if s.Waterfall.FFT != detector.SearchFFTSize {
		s.Waterfall.FFT = detector.SearchFFTSize
		changed = true
	}
	if s.ExchangeSerialNumber <= 0 {
		s.ExchangeSerialNumber = 1
		changed = true
	}
	if s.RXFrequencyHz < 200 || s.RXFrequencyHz > 5000 {
		s.RXFrequencyHz = 1500
		changed = true
	}
	if s.TXFrequencyHz < 200 || s.TXFrequencyHz > 5000 {
		s.TXFrequencyHz = 1500
		changed = true
	}
	if s.Layout.WindowWidth < minMainWindowWidth {
		s.Layout.WindowWidth = 1360
		changed = true
	}
	if s.Layout.WindowHeight < 680 {
		s.Layout.WindowHeight = 820
		changed = true
	}
	if normalizeFrequencyBands(&s) {
		changed = true
	}
	return s, changed
}

func (a *App) DomReady(ctx context.Context) {
	a.ctx = ctx
	a.emit(model.EventSettingsState, a.Settings())
	a.emit(model.EventAudioDevices, map[string]any{"inputs": []audio.Device{}, "outputs": []audio.Device{}})
	a.emit(model.EventAudioState, map[string]any{"running": false})
	a.frequencyMu.RLock()
	dial := a.dialFrequency
	a.frequencyMu.RUnlock()
	a.settingsMu.RLock()
	backend := a.settings.Radio.Backend
	pttMethod := a.settings.Radio.PTTMethod
	modelID := a.settings.Radio.RigModelID
	mode := a.settings.Radio.Mode
	a.settingsMu.RUnlock()
	a.emit(model.EventRadioState, map[string]any{"connected": false, "frequencyHz": dial, "mode": mode, "controlMode": radioControlMode(backend, pttMethod)})
	if modelID > 0 {
		a.emit(model.EventRadioSerialPorts, radio.AvailableSerialPorts())
	}
	a.emit(model.EventAppReady, map[string]any{"protocol": jtty.CurrentProtocol, "health": a.Health()})
	a.startupOnce.Do(func() { go a.deferredStartup() })
}

func (a *App) deferredStartup() {
	// Keep the first paint path free of device enumeration and CAT work.
	a.configureHotkeys()
	s := a.Settings()
	if s.AutoStartMonitor {
		time.Sleep(150 * time.Millisecond)
		if err := a.StartReceiver(); err != nil {
			a.emit(model.EventAudioState, map[string]any{"running": false, "error": err.Error()})
		}
	}
	if s.Radio.RigModelID > 0 {
		time.Sleep(250 * time.Millisecond)
		if err := a.ConnectRadio(); err != nil {
			a.emit(model.EventRadioTest, map[string]any{"ok": false, "kind": "connect", "error": err.Error()})
		}
	}
}

func (a *App) BeforeClose(ctx context.Context) bool {
	if ctx != nil {
		w, h := wailsruntime.WindowGetSize(ctx)
		if w < minMainWindowWidth {
			w = minMainWindowWidth
		}
		if h < 680 {
			h = 680
		}
		a.settingsMu.Lock()
		a.settings.Layout.WindowWidth = w
		a.settings.Layout.WindowHeight = h
		settings := a.settings
		a.settingsMu.Unlock()
		if a.configPath != "" {
			_ = storage.SaveSettings(a.configPath, settings)
		}
	}
	a.stopRadioMeterPolling()
	_ = a.StopReceiver()
	a.StopTransmit()
	if a.recorder != nil {
		_ = a.recorder.Stop()
	}
	_ = a.hotkeys.Close()
	if a.pttSerial != nil {
		_ = a.pttSerial.Close()
	}
	a.closeTestRadio(false)
	_ = a.DisconnectRadio()
	return false
}

func (a *App) Settings() model.Settings {
	a.settingsMu.RLock()
	s := a.settings
	a.settingsMu.RUnlock()
	if strings.TrimSpace(s.Audio.RecordDirectory) == "" {
		s.Audio.RecordDirectory = filepath.Join(a.dataDir(), "record")
	}
	return s
}

func (a *App) SaveSettings(settings model.Settings) error {
	// A configuration test session is transient. Close it before applying
	// settings. If the settings dialog had temporarily displaced a live radio,
	// remember that fact so the live connection is restored after the save.
	testRestoreLive := a.closeTestRadio(false)
	// Model ID is authoritative. Keep Backend only as a compatibility field
	// for old settings/telemetry.
	if settings.Radio.RigModelID <= 0 {
		settings.Radio.Backend = "none"
		settings.Radio.RigModelID = 0
		settings.Radio.RigName = "None"
		settings.Radio.PTTMethod = "VOX"
	} else {
		settings.Radio.Backend = "hamlib-rigctld"
		method := strings.ToUpper(strings.TrimSpace(settings.Radio.PTTMethod))
		if method != "VOX" && method != "CAT" && method != "DTR" && method != "RTS" {
			settings.Radio.PTTMethod = "VOX"
		}
	}
	if strings.TrimSpace(settings.Audio.RecordDirectory) == "" {
		settings.Audio.RecordDirectory = filepath.Join(a.dataDir(), "record")
	}
	settings.Waterfall.MinHz = 0
	settings.Waterfall.MaxHz = normalizeFrequencyMax(settings.Waterfall.MaxHz)
	settings.Waterfall.FFT = detector.SearchFFTSize
	settings.Waterfall.Hop = detector.SearchHop
	normalizeFrequencyBands(&settings)
	if settings.DecodeWindowLimit <= 0 {
		settings.DecodeWindowLimit = 2000
	}
	if settings.ExchangeSerialNumber <= 0 {
		settings.ExchangeSerialNumber = 1
	}
	if settings.RXFrequencyHz < 200 || settings.RXFrequencyHz > 5000 {
		settings.RXFrequencyHz = 1500
	}
	if settings.TXFrequencyHz < 200 || settings.TXFrequencyHz > 5000 {
		settings.TXFrequencyHz = 1500
	}
	if settings.Layout.WindowWidth < minMainWindowWidth {
		settings.Layout.WindowWidth = 1360
	}
	if settings.Layout.WindowHeight < 680 {
		settings.Layout.WindowHeight = 820
	}
	if settings.Layout.OperationHeight != 270 {
		settings.Layout.OperationHeight = 270
	}
	settings.LogbookPath = "JTTY.adi"
	if settings.DecodeLogMode != logbook.DecodeLogYear && settings.DecodeLogMode != logbook.DecodeLogMonth && settings.DecodeLogMode != logbook.DecodeLogSingle {
		settings.DecodeLogMode = logbook.DecodeLogSingle
	}
	if settings.Audio.SampleRate <= 0 {
		settings.Audio.SampleRate = 48000
	}
	if settings.Audio.BufferMS < 5 {
		settings.Audio.BufferMS = 5
	}
	if settings.Audio.BufferMS > 200 {
		settings.Audio.BufferMS = 200
	}
	if settings.Audio.TxAudioLevel < 0 {
		settings.Audio.TxAudioLevel = 0
	}
	if settings.Audio.TxAudioLevel > 100 {
		settings.Audio.TxAudioLevel = 100
	}
	if settings.Radio.PollIntervalSec < 1 {
		settings.Radio.PollIntervalSec = 1
	}
	if settings.Radio.PollIntervalSec > 30 {
		settings.Radio.PollIntervalSec = 30
	}
	a.settingsMu.Lock()
	old := a.settings
	// Main-window size is persisted only by the dedicated window resize/close path.
	// Keep it out of generic settings saves so a stale frontend settings snapshot
	// cannot overwrite the user's latest window dimensions.
	if old.Layout.WindowWidth > 0 {
		settings.Layout.WindowWidth = old.Layout.WindowWidth
	}
	if old.Layout.WindowHeight > 0 {
		settings.Layout.WindowHeight = old.Layout.WindowHeight
	}
	a.settings = settings
	stored := a.store
	a.settingsMu.Unlock()
	if stored == nil {
		a.settingsMu.Lock()
		a.store = decode.NewStore(settings.DecodeWindowLimit)
		stored = a.store
		a.settingsMu.Unlock()
	}
	if old.DecodeWindowLimit != settings.DecodeWindowLimit {
		stored.SetLimit(settings.DecodeWindowLimit)
	}
	if a.configPath != "" {
		if err := storage.SaveSettings(a.configPath, settings); err != nil {
			return err
		}
	}
	a.receiverMu.Lock()
	pipeline := a.pipeline
	receiverRunning := a.capture != nil
	a.receiverMu.Unlock()
	if pipeline != nil {
		pipeline.SetDecoderSettings(settings.Decoder.ThresholdDb, settings.Decoder.FrequencyToleranceHz, settings.Decoder.TrackerToleranceHz, time.Duration(settings.Decoder.TrackerTTLMS)*time.Millisecond)
	}
	if old.RecordEnabled != settings.RecordEnabled || old.Audio.RecordDirectory != settings.Audio.RecordDirectory {
		if receiverRunning {
			if err := a.configureRecorder(settings); err != nil {
				return err
			}
		}
	}
	inputChanged := old.Audio.InputDeviceID != settings.Audio.InputDeviceID || old.Audio.SampleRate != settings.Audio.SampleRate || old.Audio.InputChannel != settings.Audio.InputChannel || old.Audio.BufferMS != settings.Audio.BufferMS
	decoderChanged := old.Decoder.Threads != settings.Decoder.Threads || old.Decoder.Mode != settings.Decoder.Mode
	if inputChanged || decoderChanged {
		if receiverRunning {
			if err := a.StopReceiver(); err != nil {
				return err
			}
			if err := a.StartReceiver(); err != nil {
				return err
			}
		}
	}
	radioChanged := old.Radio != settings.Radio
	if radioChanged || testRestoreLive {
		_ = a.DisconnectRadio()
		if settings.Radio.RigModelID > 0 {
			go func() {
				if err := a.ConnectRadio(); err != nil {
					a.emit(model.EventRadioTest, map[string]any{"ok": false, "kind": "connect", "error": err.Error()})
				}
			}()
		}
	}
	a.configureHotkeys()
	a.emitRecordState()
	a.emit(model.EventSettingsState, settings)
	return nil
}

func (a *App) ListAudioDevices() map[string]any {
	a.emit(model.EventSettingsState, a.Settings())
	inputs, inErr := a.audioManager.Inputs()
	outputs, outErr := a.audioManager.Outputs()
	return map[string]any{"inputs": inputs, "outputs": outputs, "inputError": errString(inErr), "outputError": errString(outErr)}
}

func (a *App) StartReceiver() error {
	a.rxPaused.Store(false)
	a.receiverMu.Lock()
	defer a.receiverMu.Unlock()
	a.settingsMu.RLock()
	s := a.settings
	a.settingsMu.RUnlock()
	if a.capture != nil {
		return fmt.Errorf("receiver already running")
	}
	if s.RecordEnabled {
		if err := a.configureRecorder(s); err != nil {
			return err
		}
	}
	workers := s.Decoder.Threads
	if workers <= 0 {
		// Preserve the original realtime decode throughput: use the available
		// logical CPUs, capped at three workers so a normal desktop keeps one
		// or more cores available for the UI/audio stack.
		workers = runtime.NumCPU()
		if workers > 3 {
			workers = 3
		}
		if workers < 1 {
			workers = 1
		}
	}
	if workers > 3 {
		workers = 3
	}
	if workers < 1 {
		workers = 1
	}
	sampleRate := s.Audio.SampleRate
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	maxHz := normalizeFrequencyMax(s.Waterfall.MaxHz)
	scanner := detector.NewScanner(detector.ScannerConfig{SampleRate: 48000, MinFrequencyHz: 0, MaxFrequencyHz: maxHz, FFTSize: detector.SearchFFTSize, ThresholdDb: s.Decoder.ThresholdDb})
	tracker := detector.NewTracker(s.Decoder.TrackerToleranceHz, time.Duration(s.Decoder.TrackerTTLMS)*time.Millisecond)
	a.pipeline = receiver.NewPipeline(scanner, tracker, a, a, 48000, detector.SearchHop)
	a.pipeline.SetFrequencyRange(0, maxHz)
	a.frequencyMu.RLock()
	center := float64(a.rxFrequency)
	a.frequencyMu.RUnlock()
	jd := jtty.NewDecoder()
	jd.SetFrequencyTolerance(s.Decoder.FrequencyToleranceHz)
	a.jttyStream = receiver.NewJTTYStream(jd, a, center, 8, workers)
	a.pipeline.AttachJTTY(a.jttyStream)
	a.capture = a.audioManager.NewCapture()
	channels := 1
	channelMode := s.Audio.InputChannel
	if channelMode == "Both" {
		channels = 2
	}
	cfg := audio.CaptureConfig{DeviceID: s.Audio.InputDeviceID, SampleRate: sampleRate, Channels: channels, BufferMS: s.Audio.BufferMS, ChannelMode: channelMode}
	if err := a.capture.Open(context.Background(), cfg, captureFrameSink{app: a}); err != nil {
		a.capture = nil
		if a.pipeline != nil {
			a.pipeline.Close()
		}
		a.pipeline = nil
		if a.jttyStream != nil {
			a.jttyStream.Close()
			a.jttyStream = nil
		}
		if s.RecordEnabled {
			_ = a.recorder.Stop()
		}
		return err
	}
	if err := a.capture.Start(context.Background()); err != nil {
		_ = a.capture.Close()
		a.capture = nil
		if a.pipeline != nil {
			a.pipeline.Close()
		}
		a.pipeline = nil
		if a.jttyStream != nil {
			a.jttyStream.Close()
			a.jttyStream = nil
		}
		if s.RecordEnabled {
			_ = a.recorder.Stop()
		}
		return err
	}
	a.emit(model.EventAudioState, map[string]any{"running": true, "sampleRate": sampleRate})
	a.emitRecordState()
	return nil
}

func (a *App) StopReceiver() error {
	a.rxPaused.Store(false)
	a.receiverMu.Lock()
	cap := a.capture
	a.capture = nil
	pipe := a.pipeline
	a.pipeline = nil
	stream := a.jttyStream
	a.jttyStream = nil
	a.receiverMu.Unlock()
	if cap == nil {
		if pipe != nil {
			pipe.Close()
		}
		if stream != nil {
			stream.Close()
		}
		if a.recorder != nil {
			_ = a.recorder.Stop()
		}
		a.emit(model.EventAudioState, map[string]any{"running": false})
		a.emitRecordState()
		return nil
	}
	var err error
	if e := cap.Stop(); e != nil {
		err = e
	}
	if e := cap.Close(); e != nil && err == nil {
		err = e
	}
	if pipe != nil {
		pipe.Close()
	}
	if stream != nil {
		stream.Close()
	}
	if a.recorder != nil {
		if e := a.recorder.Stop(); e != nil && err == nil {
			err = e
		}
	}
	a.emit(model.EventAudioState, map[string]any{"running": false})
	a.emitRecordState()
	return err
}

func (a *App) OnPCMFrame(frame audio.PCMFrame) {
	if a.rxPaused.Load() {
		return
	}
	if len(frame.Samples) > 0 {
		var sum float64
		for _, sample := range frame.Samples {
			v := float64(sample)
			sum += v * v
		}
		rms := math.Sqrt(sum / float64(len(frame.Samples)))
		dbfs := -120.0
		if rms > 1e-9 {
			dbfs = 20 * math.Log10(rms)
		}
		dbfs = math.Max(-120, math.Min(0, dbfs))
		a.meterMu.Lock()
		shouldEmit := a.lastAudioMeter.IsZero() || time.Since(a.lastAudioMeter) >= 50*time.Millisecond
		if shouldEmit {
			a.lastAudioMeter = time.Now()
		}
		a.meterMu.Unlock()
		if shouldEmit {
			a.emit(model.EventAudioMeter, map[string]any{"dbfs": dbfs})
		}
	}
	a.receiverMu.Lock()
	pipeline := a.pipeline
	a.receiverMu.Unlock()
	if pipeline != nil {
		pipeline.Process(frame)
	}
	a.settingsMu.RLock()
	enabled := a.settings.RecordEnabled
	a.settingsMu.RUnlock()
	if enabled && a.recorder != nil {
		if err := a.recorder.WriteAsync(frame); err != nil && err.Error() != "record queue full" {
			a.emit(model.EventRecordError, map[string]any{"error": err.Error()})
		}
	}
}
func (a *App) OnCandidates(candidates []detector.Candidate) {
	if !a.allowHighRateEmit(&a.lastCandidateEmitNS, 120*time.Millisecond) {
		return
	}
	a.emit(model.EventCandidate, candidates)
}

func (a *App) OnWaterfall(frame waterfall.Frame) {
	if !a.allowHighRateEmit(&a.lastWaterfallEmitNS, 120*time.Millisecond) {
		return
	}
	a.emit(model.EventWaterfall, frame)
}

func (a *App) allowHighRateEmit(last *atomic.Int64, interval time.Duration) bool {
	now := time.Now().UnixNano()
	for {
		prev := last.Load()
		if prev != 0 && now-prev < interval.Nanoseconds() {
			return false
		}
		if last.CompareAndSwap(prev, now) {
			return true
		}
	}
}

func (a *App) OnDecodeMessage(item model.DecodeMessage) {
	a.settingsMu.RLock()
	stored := a.store
	s := a.settings
	a.settingsMu.RUnlock()
	item, added := stored.UpsertJTTY(item)
	if added {
		a.emit(model.EventDecodeAdded, item)
	} else {
		a.emit(model.EventDecodeUpdated, item)
	}

	// JTTY multi-frame messages are first shown as a partial line and then
	// updated in-place when the final frame arrives. ALL.txt must record the
	// same completed text that the operator ultimately sees, rather than the
	// first partial fragment. Single-frame messages are already marked complete.
	if s.DecodeLogEnabled && a.decodeLogger != nil && item.JTTYLastFrame {
		a.frequencyMu.RLock()
		dial := a.dialFrequency
		a.frequencyMu.RUnlock()
		if err := a.decodeLogger.Append(a.dataDir(), s.DecodeLogMode, item.SignalUTC, dial, item.SNR, item.DT, item.FrequencyHz, item.Message); err != nil {
			a.emit(model.EventLogError, map[string]any{"error": err.Error()})
		}
	}
}

func (a *App) configureHotkeys() {
	a.settingsMu.RLock()
	macros := append([]model.MacroConfig(nil), a.settings.Macros...)
	a.settingsMu.RUnlock()
	desired := make(map[string]struct{})
	for _, m := range macros {
		if m.Enabled && m.Global && strings.TrimSpace(m.Shortcut) != "" {
			desired[m.ID] = struct{}{}
		}
	}
	for id := range a.registeredHotkeys {
		if _, ok := desired[id]; !ok {
			_ = a.hotkeys.Unregister(id)
		}
	}
	for _, m := range macros {
		if _, ok := desired[m.ID]; ok {
			_ = a.hotkeys.Register(context.Background(), hotkey.Binding{ID: m.ID, Name: m.Name, Shortcut: m.Shortcut, Global: true, Enabled: true})
		}
	}
	a.registeredHotkeys = desired
}

func (a *App) RegisterGlobalHotkey(binding hotkey.Binding) error {
	return a.hotkeys.Register(context.Background(), binding)
}
func (a *App) UnregisterGlobalHotkey(id string) error { return a.hotkeys.Unregister(id) }

func (a *App) TriggerMacro(id, exchange, queueCall string) string {
	message := a.RenderMacro(id, exchange, queueCall)
	if message == "" {
		return ""
	}
	a.emit(model.EventMacroTriggered, model.MacroTriggered{ID: id, Message: message})
	return message
}

func (a *App) EnqueueTX(message string) []string {
	if message != "" {
		a.txQueue.Push(message)
	}
	items := a.txQueue.Snapshot()
	a.emit(model.EventQueueChanged, items)
	return items
}
func (a *App) PopTX() string {
	message, ok := a.txQueue.Pop()
	if !ok {
		return ""
	}
	a.emit(model.EventQueueChanged, a.txQueue.Snapshot())
	return message
}
func (a *App) ClearTXQueue() []string {
	a.txQueue.Clear()
	items := a.txQueue.Snapshot()
	a.emit(model.EventQueueChanged, items)
	return items
}

func (a *App) NotifyTXStarted(macroID, message string) {
	now := time.Now().UTC()
	a.frequencyMu.RLock()
	txHz := a.txFrequency
	dialHz := a.dialFrequency
	a.frequencyMu.RUnlock()
	a.emit(model.EventTxStarted, map[string]any{"macro": macroID, "message": message, "utc": now, "frequencyHz": txHz})
	a.emit(model.EventRadioMeter, map[string]any{"transmitting": true})
	a.settingsMu.RLock()
	enabled := a.settings.DecodeLogEnabled
	mode := a.settings.DecodeLogMode
	a.settingsMu.RUnlock()
	if enabled && a.decodeLogger != nil {
		if err := a.decodeLogger.AppendTX(a.dataDir(), mode, now, dialHz, txHz, message); err != nil {
			a.emit(model.EventLogError, map[string]any{"error": err.Error()})
		}
	}
	if strings.TrimSpace(message) != "" {
		a.noteQSOTransmit(now, message)
	}
}

func (a *App) emitTXProgress(id uint64, macroID, message string, frame, total int, elapsed, duration float64) {
	if total < 1 {
		total = 1
	}
	if duration < 0 {
		duration = 0
	}
	percent := 0.0
	if duration > 0 {
		percent = elapsed / duration * 100
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	a.emitRaw("tx:progress", map[string]any{
		"id": id, "macro": macroID, "message": message, "frame": frame, "frames": total,
		"percent": percent, "elapsedSec": elapsed, "durationSec": duration,
		"transmitting": percent < 100,
	})
}
func (a *App) NotifyTXFinished(macroID, message string) {
	a.emit(model.EventTxFinished, map[string]any{"macro": macroID, "message": message, "utc": time.Now().UTC()})
	a.emit(model.EventRadioMeter, map[string]any{"transmitting": false})
}

// noteQSOTransmit records only the first information send for a callsign.
// The timestamp is deliberately retained until the operator explicitly logs
// the QSO, so merely finishing a TX never creates an ADIF record.
func (a *App) noteQSOTransmit(at time.Time, message string) {
	call := jtty.NormalizeCallsign(a.DXCall())
	if call == "" || !messageContainsCall(message, call) {
		return
	}
	if call == "" {
		return
	}
	a.qsoMu.Lock()
	if _, exists := a.qsoStarts[call]; exists {
		a.qsoMu.Unlock()
		return
	}
	a.qsoStarts[call] = at.UTC()
	a.qsoMu.Unlock()
	a.emitRaw(string(model.EventQSOStarted), map[string]any{"call": call, "startUtc": at.UTC()})
}

func messageContainsCall(message, call string) bool {
	message = strings.ToUpper(message)
	call = strings.ToUpper(strings.TrimSpace(call))
	if call == "" {
		return false
	}
	for _, field := range strings.Fields(message) {
		if jtty.NormalizeCallsign(field) == call {
			return true
		}
	}
	return false
}

func (a *App) configureRecorder(settings model.Settings) error {
	dir := strings.TrimSpace(settings.Audio.RecordDirectory)
	if dir == "" {
		dir = filepath.Join(a.dataDir(), "record")
	}
	if !settings.RecordEnabled {
		if a.recorder != nil {
			return a.recorder.Stop()
		}
		return nil
	}
	if a.recorder == nil {
		a.recorder = audio.NewRecorder()
	}
	if err := a.recorder.Stop(); err != nil {
		return err
	}
	return a.recorder.Start(dir)
}

func (a *App) emitRecordState() {
	a.settingsMu.RLock()
	enabled := a.settings.RecordEnabled
	dir := strings.TrimSpace(a.settings.Audio.RecordDirectory)
	a.settingsMu.RUnlock()
	if dir == "" {
		dir = filepath.Join(a.dataDir(), "record")
	}
	path := ""
	active := false
	if a.recorder != nil {
		active = a.recorder.IsActive()
		path = a.recorder.CurrentPath()
	}
	a.emitRaw(string(model.EventRecordState), map[string]any{"enabled": enabled, "active": active, "directory": dir, "path": path})
}

func (a *App) updateSaveOptions(payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var v struct {
		DecodeLogEnabled *bool  `json:"decodeLogEnabled"`
		DecodeLogMode    string `json:"decodeLogMode"`
		RecordEnabled    *bool  `json:"recordEnabled"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	a.settingsMu.Lock()
	settings := a.settings
	if v.DecodeLogEnabled != nil {
		settings.DecodeLogEnabled = *v.DecodeLogEnabled
	}
	if v.DecodeLogMode != "" {
		settings.DecodeLogMode = v.DecodeLogMode
	}
	if v.RecordEnabled != nil {
		settings.RecordEnabled = *v.RecordEnabled
	}
	a.settingsMu.Unlock()
	if err := a.SaveSettings(settings); err != nil {
		return err
	}
	a.emit(model.EventSettingsState, settings)
	a.emit(model.EventSettingsSaved, map[string]any{"ok": true})
	return nil
}

func (a *App) recordQSOFromPayload(payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var q logbook.QSO
	if err := json.Unmarshal(b, &q); err != nil {
		return err
	}
	q.Call = jtty.NormalizeCallsign(q.Call)
	if q.Call == "" {
		return fmt.Errorf("DX 呼号不能为空")
	}
	if q.Mode == "" {
		q.Mode = "JTTY"
	}
	if q.Band == "" {
		a.bandMu.RLock()
		band := a.currentBand
		a.bandMu.RUnlock()
		if band != "" {
			q.Band = band
			if !strings.HasSuffix(q.Band, "m") {
				q.Band += "m"
			}
		} else {
			q.Band = "20m"
		}
	}
	if q.RSTSent == "" {
		q.RSTSent = "599"
	}
	if q.RSTRcvd == "" {
		q.RSTRcvd = "599"
	}
	if strings.TrimSpace(q.Grid) == "" {
		q.Grid = a.DXGrid()
	}
	if q.Frequency <= 0 {
		a.frequencyMu.RLock()
		q.Frequency = a.dialFrequency
		a.frequencyMu.RUnlock()
	}
	a.settingsMu.RLock()
	myCall := a.settings.MyCall
	myGrid := a.settings.MyGrid
	a.settingsMu.RUnlock()
	if q.Operator == "" {
		q.Operator = myCall
	}
	if q.StationCallsign == "" {
		q.StationCallsign = myCall
	}
	if strings.TrimSpace(q.StationGrid) == "" {
		q.StationGrid = strings.ToUpper(strings.TrimSpace(myGrid))
	}
	a.qsoMu.Lock()
	start, tracked := a.qsoStarts[q.Call]
	a.qsoMu.Unlock()
	if tracked {
		q.StartUTC = start.UTC()
	} else if q.StartUTC.IsZero() {
		return fmt.Errorf("该呼号尚未检测到首次发送信息，无法确定 QSO 开始时间")
	}
	// The UI captures the moment the operator clicks 记录通联; use that UTC
	// timestamp for TIME_OFF. Fall back to the backend event time if omitted.
	if q.EndUTC.IsZero() {
		q.EndUTC = time.Now().UTC()
	} else {
		q.EndUTC = q.EndUTC.UTC()
	}
	path := filepath.Join(a.dataDir(), "JTTY.adi")
	if err := logbook.AppendADIF(path, q); err != nil {
		return err
	}
	a.qsoMu.Lock()
	delete(a.qsoStarts, q.Call)
	a.qsoMu.Unlock()
	a.emit(model.EventQSOLogged, q)
	a.emit(model.EventQSORecordResult, map[string]any{"ok": true, "qso": q})
	return nil
}

func (a *App) emitRaw(event string, payload any) {
	if a.sink != nil {
		a.sink.Emit(event, payload)
	}
}

func (a *App) dataDir() string {
	if a.configPath != "" {
		return filepath.Dir(a.configPath)
	}
	home, err := os.UserConfigDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, "JTTY-Go")
}

func (a *App) DeleteALLLogs() error {
	dir := a.dataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	paths := []string{filepath.Join(dir, "ALL.txt"), filepath.Join(dir, "all.log")}
	if matches, err := filepath.Glob(filepath.Join(dir, "ALL-*.txt")); err == nil {
		paths = append(paths, matches...)
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (a *App) DeleteADIF() error {
	path := filepath.Join(a.dataDir(), "JTTY.adi")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (a *App) OpenDataDirectory() error {
	dir := a.dataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer.exe", dir).Start()
	case "darwin":
		return exec.Command("open", dir).Start()
	default:
		return exec.Command("xdg-open", dir).Start()
	}
}

func (a *App) OpenADIFLog() error {
	path := filepath.Join(a.dataDir(), "JTTY.adi")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, createErr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
		if createErr != nil {
			return createErr
		}
		if _, writeErr := f.WriteString("Generated by JTTY-Go<eoh>\n"); writeErr != nil {
			_ = f.Close()
			return writeErr
		}
		if closeErr := f.Close(); closeErr != nil {
			return closeErr
		}
	} else if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

func (a *App) bundledHamlibDir() string {
	// Wails places the final executable in its build/bin directory on Windows.
	// The bundled Hamlib runtime is released beside JTTY-Go.exe so rigctld.exe
	// and all of its DLL dependencies share one deterministic search directory.
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func (a *App) ensureBundledHamlib() error {
	if a.embeddedAssets == nil {
		return nil
	}
	return radio.InstallEmbeddedHamlib(a.embeddedAssets, a.bundledHamlibDir())
}

func (a *App) hamlibExecutable() string {
	_ = a.ensureBundledHamlib()
	base := a.bundledHamlibDir()
	// The released application always prefers its bundled rigctld. The
	// configured path is retained only as a development fallback when the
	// bundled binary is not present.
	bundled := filepath.Join(base, "rigctld.exe")
	if fi, err := os.Stat(bundled); err == nil && !fi.IsDir() {
		return bundled
	}
	a.settingsMu.RLock()
	path := strings.TrimSpace(a.settings.Radio.RigctldPath)
	a.settingsMu.RUnlock()
	if path != "" {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return path
		}
		if resolved, err := exec.LookPath(path); err == nil {
			return resolved
		}
	}
	return ""
}

func (a *App) hamlibInstallDir() string {
	if exe := a.hamlibExecutable(); exe != "" {
		return filepath.Dir(exe)
	}
	return a.bundledHamlibDir()
}

func prependHamlibPath(env []string, dir string) []string {
	if dir == "" {
		return env
	}
	pathValue := dir
	found := false
	for _, item := range env {
		if strings.HasPrefix(strings.ToUpper(item), "PATH=") {
			pathValue += string(os.PathListSeparator) + item[5:]
			found = true
			break
		}
	}
	if !found {
		return append(env, "PATH="+pathValue)
	}
	out := make([]string, 0, len(env))
	for _, item := range env {
		if strings.HasPrefix(strings.ToUpper(item), "PATH=") {
			out = append(out, "PATH="+pathValue)
		} else {
			out = append(out, item)
		}
	}
	return out
}

func (a *App) resolveRigctldForConfig(cfg model.RadioSettings) string {
	_ = a.ensureBundledHamlib()
	base := a.bundledHamlibDir()
	bundled := filepath.Join(base, "rigctld.exe")
	if fi, err := os.Stat(bundled); err == nil && !fi.IsDir() {
		return bundled
	}
	path := strings.TrimSpace(cfg.RigctldPath)
	if path != "" {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return path
		}
		if resolved, err := exec.LookPath(path); err == nil {
			return resolved
		}
	}
	return ""
}

func (a *App) startRigctld(cfg model.RadioSettings, track bool) (*exec.Cmd, bool) {
	executable := a.resolveRigctldForConfig(cfg)
	if executable == "" {
		return nil, false
	}
	host := cfg.Host
	if host == "" {
		host = "127.0.0.1"
	}
	if host != "127.0.0.1" && host != "localhost" {
		return nil, false
	}
	tcpPort := cfg.Port
	if tcpPort <= 0 {
		tcpPort = 4532
	}
	serialPort := strings.TrimSpace(cfg.SerialPort)
	if serialPort == "" {
		ports := radio.AvailableSerialPorts()
		if len(ports) > 0 {
			serialPort = ports[0]
		}
	}
	if serialPort == "" {
		return nil, false
	}
	args := buildRigctldArgs(cfg, host, tcpPort, serialPort)
	cmd := newHiddenCommand(executable, args...)
	cmd.Dir = filepath.Dir(executable)
	cmd.Env = prependHamlibPath(cmd.Environ(), filepath.Dir(executable))
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, false
	}
	if track {
		a.radioMu.Lock()
		old := a.rigctldCmd
		a.rigctldCmd = cmd
		a.radioMu.Unlock()
		if old != nil && old.Process != nil && old != cmd {
			_ = old.Process.Kill()
			_ = old.Wait()
		}
	}
	return cmd, true
}

// buildRigctldArgs mirrors the relevant WSJT-X/Hamlib serial and PTT
// configuration semantics while retaining JTTY-Go's local rigctld architecture.
// Hamlib accepts backend-specific parameters through --set-conf/-C.
func buildRigctldArgs(cfg model.RadioSettings, host string, tcpPort int, serialPort string) []string {
	baud := cfg.Baud
	if baud <= 0 {
		baud = 9600
	}
	dataBits := cfg.DataBits
	if dataBits != 7 && dataBits != 8 {
		dataBits = 8
	}
	stopBits := cfg.StopBits
	if stopBits != 1 && stopBits != 2 {
		stopBits = 1
	}
	handshake := strings.TrimSpace(cfg.Handshake)
	switch strings.ToLower(handshake) {
	case "xon/xoff", "xonxoff":
		handshake = "XONXOFF"
	case "hardware":
		handshake = "Hardware"
	default:
		handshake = "None"
	}

	args := []string{
		"-m", fmt.Sprintf("%d", cfg.RigModelID),
		"-r", serialPort,
		"-s", fmt.Sprintf("%d", baud),
		"-T", host,
		"-t", fmt.Sprintf("%d", tcpPort),
		"-C", fmt.Sprintf("data_bits=%d,stop_bits=%d,serial_handshake=%s", dataBits, stopBits, handshake),
	}

	// Match WSJT-X's PTT model: VOX disables Hamlib PTT, CAT leaves the
	// backend's CAT PTT type untouched, and DTR/RTS explicitly select the
	// corresponding serial line. A separate PTT port is only supplied when it
	// is actually different from the CAT port.
	pttMethod := strings.ToUpper(strings.TrimSpace(cfg.PTTMethod))
	switch pttMethod {
	case "VOX", "":
		args = append(args, "-C", "ptt_type=None")
	case "DTR", "RTS":
		args = append(args, "-C", "ptt_type="+pttMethod)
		pttPort := strings.TrimSpace(cfg.PTTSerialPort)
		if pttPort != "" && !strings.EqualFold(pttPort, serialPort) {
			args = append(args, "-C", "ptt_pathname="+pttPort)
		}
	}

	// Force control-line states are separate from PTT selection. For RTS we
	// follow WSJT-X's rule not to fight a hardware-handshake configuration.
	forceDTR := strings.ToLower(strings.TrimSpace(cfg.ForceDTR))
	if forceDTR == "on" || forceDTR == "off" {
		args = append(args, "-C", "dtr_state="+strings.ToUpper(forceDTR))
	}
	forceRTS := strings.ToLower(strings.TrimSpace(cfg.ForceRTS))
	if (forceRTS == "on" || forceRTS == "off") && handshake != "Hardware" {
		args = append(args, "-C", "rts_state="+strings.ToUpper(forceRTS))
	}
	return args
}

func (a *App) maybeStartRigctld(cfg model.RadioSettings) bool {
	_, ok := a.startRigctld(cfg, true)
	return ok
}

func (a *App) stopRadioPolling() {
	a.radioMu.Lock()
	cancel := a.pollCancel
	a.pollCancel = nil
	a.radioMu.Unlock()
	if cancel != nil {
		cancel()
		a.pollWG.Wait()
	}
}
func (a *App) startRadioPolling() {
	a.stopRadioPolling()
	a.settingsMu.RLock()
	sec := a.settings.Radio.PollIntervalSec
	a.settingsMu.RUnlock()
	if sec < 1 {
		sec = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.radioMu.Lock()
	a.pollCancel = cancel
	a.pollWG.Add(1)
	a.radioMu.Unlock()
	go func() {
		defer a.pollWG.Done()
		ticker := time.NewTicker(time.Duration(sec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.pollRadioState()
			}
		}
	}()
}
func (a *App) pollRadioState() {
	a.radioMu.Lock()
	r := a.radio
	a.radioMu.Unlock()
	if r == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	freq, fe := r.Frequency(ctx)
	mode, me := r.Mode(ctx)
	a.settingsMu.RLock()
	backend := a.settings.Radio.Backend
	pttMethod := a.settings.Radio.PTTMethod
	a.settingsMu.RUnlock()
	st := map[string]any{"connected": fe == nil && me == nil, "frequencyHz": freq, "mode": mode, "controlMode": radioControlMode(backend, pttMethod)}
	if fe != nil {
		st["frequencyError"] = fe.Error()
	}
	if me != nil {
		st["modeError"] = me.Error()
	}
	if fe == nil {
		a.frequencyMu.Lock()
		a.dialFrequency = freq
		a.frequencyMu.Unlock()
	}
	a.emit(model.EventRadioState, st)
}
func (a *App) setPTT(on bool) error {
	a.settingsMu.RLock()
	method := strings.ToUpper(a.settings.Radio.PTTMethod)
	port := a.settings.Radio.PTTSerialPort
	a.settingsMu.RUnlock()
	if method == "VOX" || method == "" {
		return nil
	}
	if method == "DTR" || method == "RTS" {
		if a.pttSerial == nil {
			return fmt.Errorf("PTT 串口控制器不可用")
		}
		if strings.TrimSpace(port) == "" {
			ports := radio.AvailableSerialPorts()
			if len(ports) > 0 {
				port = ports[0]
			}
		}
		if err := a.pttSerial.Set(port, method, on); err != nil {
			return err
		}
		return a.applyForcedControlLines(method)
	}
	a.radioMu.Lock()
	r := a.radio
	a.radioMu.Unlock()
	if r == nil {
		return fmt.Errorf("电台尚未连接")
	}
	if err := r.PTT(context.Background(), on); err != nil {
		return err
	}
	return a.applyForcedControlLines(method)
}

func (a *App) applyForcedControlLines(pttMethod string) error {
	if a.pttSerial == nil {
		return nil
	}
	a.settingsMu.RLock()
	port := a.settings.Radio.PTTSerialPort
	forceDTR := strings.ToLower(strings.TrimSpace(a.settings.Radio.ForceDTR))
	forceRTS := strings.ToLower(strings.TrimSpace(a.settings.Radio.ForceRTS))
	a.settingsMu.RUnlock()
	if strings.TrimSpace(port) == "" {
		return nil
	}
	// Do not override the line currently being used as the active PTT source.
	if pttMethod != "DTR" {
		if forceDTR == "on" || forceDTR == "off" {
			if err := a.pttSerial.Set(port, "DTR", forceDTR == "on"); err != nil {
				return err
			}
		}
	}
	if pttMethod != "RTS" {
		if forceRTS == "on" || forceRTS == "off" {
			if err := a.pttSerial.Set(port, "RTS", forceRTS == "on"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *App) ConnectRadio() error {
	if err := a.ensureBundledHamlib(); err != nil {
		return fmt.Errorf("准备内置 Hamlib 失败: %w", err)
	}
	a.settingsMu.RLock()
	cfg := a.settings.Radio
	a.settingsMu.RUnlock()
	if cfg.RigModelID <= 0 {
		return fmt.Errorf("未选择电台")
	}
	var r radio.Radio
	r = radio.NewRigctld(radio.RigctldConfig{Address: cfg.Host, Port: cfg.Port, Mode: cfg.Mode, PassbandHz: cfg.PassbandHz})
	if err := r.Open(context.Background()); err != nil {
		if !a.maybeStartRigctld(cfg) {
			return err
		}
		last := err
		for i := 0; i < 20; i++ {
			time.Sleep(100 * time.Millisecond)
			if e := r.Open(context.Background()); e == nil {
				last = nil
				break
			} else {
				last = e
			}
		}
		if last != nil {
			a.radioMu.Lock()
			cmd := a.rigctldCmd
			a.rigctldCmd = nil
			a.radioMu.Unlock()
			if cmd != nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
			return last
		}
	}
	a.radioMu.Lock()
	old := a.radio
	a.radio = r
	a.radioMu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	if cfg.Mode != "" {
		_ = r.SetMode(context.Background(), cfg.Mode)
	}
	freq, fe := r.Frequency(context.Background())
	mode, me := r.Mode(context.Background())
	if fe == nil {
		a.frequencyMu.Lock()
		a.dialFrequency = freq
		a.frequencyMu.Unlock()
	}
	if cfg.SplitMode == "Rig" {
		if sr, ok := r.(interface {
			SetSplit(context.Context, string) error
			SetSplitFrequency(context.Context, int64) error
		}); ok {
			_ = sr.SetSplit(context.Background(), "VFOB")
			if fe == nil {
				_ = sr.SetSplitFrequency(context.Background(), freq)
			}
		}
	}
	if cfg.PTTMethod == "DTR" || cfg.PTTMethod == "RTS" {
		if err := a.setPTT(false); err != nil {
			return err
		}
	} else if err := a.applyForcedControlLines(strings.ToUpper(cfg.PTTMethod)); err != nil {
		return err
	}
	st := map[string]any{"connected": true, "frequencyHz": freq, "mode": mode, "controlMode": radioControlMode(cfg.Backend, cfg.PTTMethod)}
	if fe != nil {
		st["frequencyError"] = fe.Error()
	}
	if me != nil {
		st["modeError"] = me.Error()
	}
	a.emit(model.EventRadioState, st)
	a.startRadioMeterPolling(r)
	a.startRadioPolling()
	return nil
}

func (a *App) DisconnectRadio() error {
	a.stopRadioPolling()
	a.stopRadioMeterPolling()
	_ = a.setPTT(false)
	a.radioMu.Lock()
	r := a.radio
	a.radio = nil
	cmd := a.rigctldCmd
	a.rigctldCmd = nil
	a.radioMu.Unlock()
	var err error
	if r != nil {
		err = r.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	a.settingsMu.RLock()
	backend := a.settings.Radio.Backend
	pttMethod := a.settings.Radio.PTTMethod
	mode := a.settings.Radio.Mode
	a.settingsMu.RUnlock()
	a.frequencyMu.RLock()
	dial := a.dialFrequency
	a.frequencyMu.RUnlock()
	a.emit(model.EventRadioState, map[string]any{"connected": false, "frequencyHz": dial, "mode": mode, "controlMode": radioControlMode(backend, pttMethod)})
	return err
}
func (a *App) startRadioMeterPolling(r radio.Radio) {
	a.stopRadioMeterPolling()
	reader, ok := r.(radio.MeterReader)
	if !ok {
		a.emit(model.EventRadioMeterCapabilities, map[string]any{"strength": false, "alc": false, "powerWatts": false, "powerPercent": false, "swr": false})
		return
	}
	capsCtx, cancelCaps := context.WithTimeout(context.Background(), 3*time.Second)
	caps, err := reader.SupportedLevels(capsCtx)
	cancelCaps()
	if err != nil {
		caps = map[string]bool{}
	}
	a.emit(model.EventRadioMeterCapabilities, map[string]any{
		"strength":     caps["STRENGTH"],
		"alc":          caps["ALC"],
		"powerWatts":   caps["RFPOWER_METER_WATTS"],
		"powerPercent": caps["RFPOWER_METER"],
		"swr":          caps["SWR"],
	})
	a.settingsMu.RLock()
	sec := a.settings.Radio.PollIntervalSec
	a.settingsMu.RUnlock()
	if sec < 1 {
		sec = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.meterMu.Lock()
	a.meterCancel = cancel
	a.meterMu.Unlock()
	a.meterWG.Add(1)
	go func() {
		defer a.meterWG.Done()
		ticker := time.NewTicker(time.Duration(sec) * time.Second)
		defer ticker.Stop()
		for {
			a.pollRadioMeters(ctx, reader, caps)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (a *App) stopRadioMeterPolling() {
	a.meterMu.Lock()
	cancel := a.meterCancel
	a.meterCancel = nil
	a.meterMu.Unlock()
	if cancel != nil {
		cancel()
		a.meterWG.Wait()
	}
	a.emit(model.EventRadioMeterCapabilities, map[string]any{"strength": false, "alc": false, "powerWatts": false, "powerPercent": false, "swr": false})
	a.emit(model.EventRadioMeter, map[string]any{"rxDb": nil, "alc": nil, "powerWatts": nil, "powerPercent": nil, "swr": nil, "transmitting": false})
}

func (a *App) pollRadioMeters(ctx context.Context, reader radio.MeterReader, caps map[string]bool) {
	a.radioMu.Lock()
	r := a.radio
	a.radioMu.Unlock()
	if r == nil {
		return
	}
	tx := r.Transmitting()
	payload := map[string]any{"transmitting": tx}
	read := func(name string) (float64, bool) {
		v, err := reader.Level(ctx, name)
		return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
	}
	if !tx && caps["STRENGTH"] {
		if v, ok := read("STRENGTH"); ok {
			payload["rxDb"] = v
		}
	}
	if tx && caps["ALC"] {
		if v, ok := read("ALC"); ok {
			payload["alc"] = v
		}
	}
	if caps["RFPOWER_METER_WATTS"] {
		if v, ok := read("RFPOWER_METER_WATTS"); ok && v >= 0 {
			payload["powerWatts"] = v
		}
	} else if caps["RFPOWER_METER"] {
		if v, ok := read("RFPOWER_METER"); ok && v >= 0 {
			payload["powerPercent"] = v
		}
	}
	if caps["SWR"] {
		if v, ok := read("SWR"); ok && v >= 0 {
			payload["swr"] = v
		}
	}
	a.emit(model.EventRadioMeter, payload)
}

func (a *App) TestRadio() map[string]any {
	a.settingsMu.RLock()
	cfg := a.settings.Radio
	a.settingsMu.RUnlock()
	return a.testRadioConfig(cfg)
}

// TestRadioSettings follows WSJT-X's configuration-dialog behavior. The CAT
// test uses the uncommitted settings from the dialog, opens the rig and keeps
// that test connection alive while the dialog remains open. This allows the
// subsequent Test PTT operation to use the same live rig session.
func (a *App) TestRadioSettings(settings model.Settings) map[string]any {
	return a.testRadioConfig(settings.Radio)
}

func testRadioConfigEqual(aCfg, bCfg model.RadioSettings) bool {
	return aCfg.RigModelID == bCfg.RigModelID &&
		strings.EqualFold(strings.TrimSpace(aCfg.SerialPort), strings.TrimSpace(bCfg.SerialPort)) &&
		aCfg.Baud == bCfg.Baud && aCfg.DataBits == bCfg.DataBits && aCfg.StopBits == bCfg.StopBits &&
		strings.EqualFold(strings.TrimSpace(aCfg.Handshake), strings.TrimSpace(bCfg.Handshake)) &&
		strings.EqualFold(strings.TrimSpace(aCfg.PTTMethod), strings.TrimSpace(bCfg.PTTMethod)) &&
		strings.EqualFold(strings.TrimSpace(aCfg.PTTSerialPort), strings.TrimSpace(bCfg.PTTSerialPort)) &&
		strings.EqualFold(strings.TrimSpace(aCfg.ForceDTR), strings.TrimSpace(bCfg.ForceDTR)) &&
		strings.EqualFold(strings.TrimSpace(aCfg.ForceRTS), strings.TrimSpace(bCfg.ForceRTS)) &&
		strings.EqualFold(strings.TrimSpace(aCfg.Host), strings.TrimSpace(bCfg.Host)) &&
		aCfg.Port == bCfg.Port && strings.EqualFold(strings.TrimSpace(aCfg.Mode), strings.TrimSpace(bCfg.Mode)) &&
		aCfg.PassbandHz == bCfg.PassbandHz
}

func (a *App) closeTestRadio(restoreLive bool) bool {
	a.radioTestMu.Lock()
	r := a.testRadio
	cmd := a.testRigctldCmd
	cfg := a.testRadioCfg
	wasLive := a.testRestoreLiveRadio
	a.testRadio = nil
	a.testRigctldCmd = nil
	a.testRadioCfg = model.RadioSettings{}
	a.testRadioOnline = false
	a.testRestoreLiveRadio = false
	a.radioTestMu.Unlock()

	if method := strings.ToUpper(strings.TrimSpace(cfg.PTTMethod)); method == "DTR" || method == "RTS" {
		_ = a.setSerialPTTForConfig(cfg, method, false)
		_ = a.applyForcedControlLinesConfig(cfg, method)
	} else if r != nil {
		_ = r.PTT(context.Background(), false)
	}
	if r != nil {
		_ = r.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}

	if restoreLive && wasLive {
		go func() {
			if err := a.ConnectRadio(); err != nil {
				a.emit(model.EventRadioTest, map[string]any{"ok": false, "kind": "connect", "error": err.Error()})
			}
		}()
	}
	return wasLive
}

func (a *App) storeTestRadio(r radio.Radio, cmd *exec.Cmd, cfg model.RadioSettings) {
	a.radioTestMu.Lock()
	a.testRadio = r
	a.testRigctldCmd = cmd
	a.testRadioCfg = cfg
	a.testRadioOnline = true
	a.radioTestMu.Unlock()
}

func (a *App) openTestRadio(cfg model.RadioSettings) (radio.Radio, *exec.Cmd, map[string]any) {
	if cfg.RigModelID <= 0 {
		return nil, nil, map[string]any{"ok": false, "error": "未选择 Hamlib 电台设备"}
	}

	wasRestorePending := a.closeTestRadio(false)
	a.radioMu.Lock()
	live := a.radio
	a.radioMu.Unlock()
	if live != nil {
		wasRestorePending = true
		_ = a.DisconnectRadio()
	}
	a.radioTestMu.Lock()
	a.testRestoreLiveRadio = wasRestorePending
	a.radioTestMu.Unlock()

	r := radio.NewRigctld(radio.RigctldConfig{
		Address:    cfg.Host,
		Port:       cfg.Port,
		Mode:       cfg.Mode,
		PassbandHz: cfg.PassbandHz,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	started := false
	if err := r.Open(ctx); err != nil {
		var startOK bool
		cmd, startOK = a.startRigctld(cfg, false)
		if !startOK {
			_ = r.Close()
			return nil, nil, map[string]any{"ok": false, "error": err.Error()}
		}
		started = true
		last := err
		for i := 0; i < 40; i++ {
			openErr := r.Open(ctx)
			if openErr == nil {
				last = nil
				break
			}
			last = openErr
			time.Sleep(100 * time.Millisecond)
		}
		if last != nil {
			_ = r.Close()
			if cmd != nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
			return nil, nil, map[string]any{"ok": false, "error": last.Error()}
		}
	}

	freq, fe := r.Frequency(ctx)
	mode, me := r.Mode(ctx)
	if fe != nil || me != nil {
		_ = r.Close()
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		res := map[string]any{"ok": false, "frequencyHz": freq, "mode": mode, "temporaryRigctld": started}
		if fe != nil {
			res["frequencyError"] = fe.Error()
		}
		if me != nil {
			res["modeError"] = me.Error()
		}
		if fe != nil {
			res["error"] = fe.Error()
		} else if me != nil {
			res["error"] = me.Error()
		}
		return nil, nil, res
	}

	a.storeTestRadio(r, cmd, cfg)
	return r, cmd, map[string]any{
		"ok":               true,
		"online":           true,
		"frequencyHz":      freq,
		"mode":             mode,
		"temporaryRigctld": started,
		"pttEligible":      testPTTEligible(cfg),
	}
}

func testPTTEligible(cfg model.RadioSettings) bool {
	switch strings.ToUpper(strings.TrimSpace(cfg.PTTMethod)) {
	case "DTR", "RTS":
		return true // WSJT-X allows direct serial PTT tests even with rig=None.
	case "CAT":
		return cfg.RigModelID > 0
	default:
		return false
	}
}

func (a *App) testRadioConfig(cfg model.RadioSettings) map[string]any {
	_, _, res := a.openTestRadio(cfg)
	return res
}

func (a *App) setSerialPTTForConfig(cfg model.RadioSettings, method string, on bool) error {
	if a.pttSerial == nil {
		return fmt.Errorf("PTT 串口控制器不可用")
	}
	port := strings.TrimSpace(cfg.PTTSerialPort)
	if port == "" {
		ports := radio.AvailableSerialPorts()
		if len(ports) > 0 {
			port = ports[0]
		}
	}
	if port == "" {
		return fmt.Errorf("没有可用的串行端口")
	}
	return a.pttSerial.Set(port, method, on)
}

func (a *App) applyForcedControlLinesConfig(cfg model.RadioSettings, pttMethod string) error {
	if a.pttSerial == nil {
		return nil
	}
	port := strings.TrimSpace(cfg.PTTSerialPort)
	if port == "" {
		return nil
	}
	forceDTR := strings.ToLower(strings.TrimSpace(cfg.ForceDTR))
	forceRTS := strings.ToLower(strings.TrimSpace(cfg.ForceRTS))
	if pttMethod != "DTR" && (forceDTR == "on" || forceDTR == "off") {
		if err := a.pttSerial.Set(port, "DTR", forceDTR == "on"); err != nil {
			return err
		}
	}
	if pttMethod != "RTS" && (forceRTS == "on" || forceRTS == "off") {
		if err := a.pttSerial.Set(port, "RTS", forceRTS == "on"); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) testPTTConfig(cfg model.RadioSettings, on bool) map[string]any {
	result := map[string]any{"ok": false, "on": on, "pttEligible": testPTTEligible(cfg)}
	method := strings.ToUpper(strings.TrimSpace(cfg.PTTMethod))
	if method == "VOX" || method == "" {
		result["error"] = "VOX 不通过独立 PTT 控制线测试，请使用 Test CAT/实际音频触发 VOX"
		return result
	}
	if method != "CAT" && method != "DTR" && method != "RTS" {
		result["error"] = "不支持的 PTT 方法: " + method
		return result
	}
	if cfg.RigModelID <= 0 && (method == "DTR" || method == "RTS") {
		if err := a.setSerialPTTForConfig(cfg, method, on); err != nil {
			result["error"] = err.Error()
			return result
		}
		// Keep the active serial-test state so Test PTT remains a toggle and
		// closing the dialog can always force the line back to RX.
		a.radioTestMu.Lock()
		a.testRadioCfg = cfg
		a.testRadioOnline = true
		a.radioTestMu.Unlock()
		result["ok"] = true
		result["online"] = true
		result["method"] = method
		return result
	}
	if cfg.RigModelID <= 0 {
		result["error"] = "未选择 Hamlib 电台设备"
		return result
	}

	a.radioTestMu.Lock()
	r := a.testRadio
	testCfg := a.testRadioCfg
	online := a.testRadioOnline
	a.radioTestMu.Unlock()
	if r == nil || !online || !testRadioConfigEqual(testCfg, cfg) {
		_, _, catResult := a.openTestRadio(cfg)
		if !boolValue(catResult["ok"]) {
			if errText := firstErrorText(catResult); errText != "" {
				result["error"] = errText
			} else {
				result["error"] = "电台连接失败"
			}
			return result
		}
		a.radioTestMu.Lock()
		r = a.testRadio
		a.radioTestMu.Unlock()
	}

	var err error
	switch method {
	case "CAT":
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err = r.PTT(ctx, on)
		cancel()
	case "DTR", "RTS":
		err = a.setSerialPTTForConfig(cfg, method, on)
		if err == nil {
			err = a.applyForcedControlLinesConfig(cfg, method)
		}
	}
	if err != nil {
		result["error"] = err.Error()
		return result
	}
	result["ok"] = true
	result["method"] = method
	result["frequencyHz"], _ = r.Frequency(context.Background())
	result["mode"], _ = r.Mode(context.Background())
	return result
}

func boolValue(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

func firstErrorText(m map[string]any) string {
	for _, key := range []string{"error", "frequencyError", "modeError"} {
		if s, ok := m[key].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func (a *App) prepareReceiverForTX() bool {
	if a.duplexMode.Load() {
		return false
	}
	a.receiverMu.Lock()
	pipeline := a.pipeline
	active := a.capture != nil && pipeline != nil
	a.receiverMu.Unlock()
	if !active {
		return false
	}
	a.rxPaused.Store(true)
	pipeline.Pause()
	return true
}

func (a *App) resumeReceiverAfterTX(resume bool) {
	if !resume {
		return
	}
	if a.duplexMode.Load() {
		a.rxPaused.Store(false)
		return
	}
	a.receiverMu.Lock()
	pipeline := a.pipeline
	active := a.capture != nil && pipeline != nil
	a.receiverMu.Unlock()
	if !active {
		a.rxPaused.Store(false)
		return
	}
	pipeline.Resume()
	a.rxPaused.Store(false)
}

func (a *App) transmitJTTY(macroID, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	frames, canonical, err := jtty.PackMessage(message, jtty.ExchangeUnknown)
	if err != nil {
		a.emitRaw("tx:error", map[string]any{"error": err.Error()})
		return
	}
	if len(frames) == 0 {
		return
	}
	// Validate the exact frames before opening the audio/PTT path. This keeps
	// UI macros, source packing, and physical transmission on the same canonical
	// message and catches an encoder regression before RF is keyed.
	decoded, _, final, valid := jtty.UnpackMessage(frames)
	if !valid || !final || decoded != canonical {
		a.emitRaw("tx:error", map[string]any{"error": "JTTY 编码校验失败", "canonical": canonical, "decoded": decoded})
		return
	}
	message = canonical
	a.emitRaw("tx:encoded", map[string]any{"message": canonical, "frames": len(frames)})
	a.txMu.Lock()
	if a.txCancel != nil {
		a.txMu.Unlock()
		a.emitRaw("tx:error", map[string]any{"error": "已有发射正在进行"})
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.txID++
	id := a.txID
	a.txCancel = cancel
	a.txKind = "message"
	a.txMu.Unlock()
	resumeReceiver := a.prepareReceiverForTX()
	defer func() {
		a.txMu.Lock()
		if a.txID == id {
			a.txCancel = nil
			a.txKind = ""
		}
		a.txMu.Unlock()
		a.resumeReceiverAfterTX(resumeReceiver)
	}()
	a.settingsMu.RLock()
	as := a.settings.Audio
	a.settingsMu.RUnlock()
	// JTTY encodes onto the radio's audio input. The TX marker selected
	// by the operator is the audio carrier frequency used by the waveform
	// generator. The RF dial frequency remains a separate CAT setting.
	a.frequencyMu.RLock()
	txHz := float64(a.txFrequency)
	a.frequencyMu.RUnlock()
	if txHz < 200 || txHz > 5000 {
		txHz = 1500
	}
	out := a.audioManager.NewOutput()
	a.txMu.Lock()
	a.audioOutput = out
	a.txMu.Unlock()
	defer func() {
		_ = out.Stop()
		_ = out.Close()
		a.txMu.Lock()
		if a.txID == id {
			a.audioOutput = nil
		}
		a.txMu.Unlock()
	}()
	if err := out.Open(ctx, audio.OutputConfig{DeviceID: as.OutputDeviceID, SampleRate: 12000, Channels: 1, BufferMS: as.BufferMS, Channel: as.OutputChannel, Level: as.TxAudioLevel}); err != nil {
		a.emitRaw("tx:error", map[string]any{"error": err.Error()})
		return
	}
	if err := a.setPTT(true); err != nil {
		a.emitRaw("tx:error", map[string]any{"error": err.Error()})
		return
	}
	started := false
	completed := false
	defer func() {
		a.txMu.Lock()
		active := a.txID == id
		a.txMu.Unlock()
		if active {
			_ = a.setPTT(false)
		}
		if !started {
			return
		}
		if completed {
			a.NotifyTXFinished(macroID, message)
			return
		}
		a.emitRaw("tx:stopped", map[string]any{"macro": macroID, "message": message, "utc": time.Now().UTC()})
		a.emit(model.EventRadioMeter, map[string]any{"transmitting": false})
	}()
	// Notify only when the real audio/ PTT transaction begins; this is the QSO START trigger.
	a.NotifyTXStarted(macroID, message)
	started = true
	frameDuration := float64(59*384) / 12000.0
	totalDuration := frameDuration * float64(len(frames))
	progressDone := make(chan struct{})
	progressStart := time.Now()
	go func() {
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-progressDone:
				return
			case <-ctx.Done():
				return
			case now := <-t.C:
				a.emitTXProgress(id, macroID, message, 0, len(frames), now.Sub(progressStart).Seconds(), totalDuration)
			}
		}
	}()
	defer close(progressDone)
	a.emitTXProgress(id, macroID, message, 0, len(frames), 0, totalDuration)
	for frameIndex, pf := range frames {
		a.emitTXProgress(id, macroID, message, frameIndex, len(frames), time.Since(progressStart).Seconds(), totalDuration)
		bits := [jtty.PayloadBits]int{}
		for i := 0; i < jtty.PayloadBits; i++ {
			if pf.Bits&(uint64(1)<<uint(jtty.PayloadBits-1-i)) != 0 {
				bits[i] = 1
			}
		}
		tones := jtty.EncodeFrame(bits)
		waveLen := len(tones) * 384
		if cap(a.txWaveScratch) < waveLen {
			a.txWaveScratch = make([]float32, waveLen)
		}
		if cap(a.txDphiScratch) < (len(tones)+2)*384 {
			a.txDphiScratch = make([]float64, (len(tones)+2)*384)
		}
		if cap(a.txPulseScratch) < 3*384 {
			a.txPulseScratch = make([]float64, 3*384)
		}
		wave := a.txWaveScratch[:waveLen]
		jtty.GenerateJTTYWaveformInto(wave, a.txDphiScratch, a.txPulseScratch, tones[:], 384, 2.0, 12000, txHz)
		if err := out.Play(ctx, wave, 12000); err != nil {
			if ctx.Err() != nil {
				return
			}
			a.emitRaw("tx:error", map[string]any{"error": err.Error()})
			return
		}
		a.emitTXProgress(id, macroID, message, frameIndex+1, len(frames), frameDuration*float64(frameIndex+1), totalDuration)
	}
	a.emitTXProgress(id, macroID, message, len(frames), len(frames), totalDuration, totalDuration)
	completed = true
}
func (a *App) tuneJTTY(freq int) {
	a.txMu.Lock()
	if a.txCancel != nil {
		a.txMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.txID++
	id := a.txID
	a.txCancel = cancel
	a.txKind = "tune"
	a.txMu.Unlock()
	resumeReceiver := a.prepareReceiverForTX()
	defer func() {
		a.txMu.Lock()
		if a.txID == id {
			a.txCancel = nil
			a.txKind = ""
		}
		a.txMu.Unlock()
		a.resumeReceiverAfterTX(resumeReceiver)
		a.emitRaw("tx:tune-state", map[string]any{"active": false, "frequencyHz": 1500})
		a.emitRaw("tx:reset", map[string]any{"reason": "tune-ended"})
	}()
	// Tune always uses a clean 1500 Hz sine-wave carrier.
	freq = 1500
	a.settingsMu.RLock()
	as := a.settings.Audio
	a.settingsMu.RUnlock()
	out := a.audioManager.NewOutput()
	a.txMu.Lock()
	a.audioOutput = out
	a.txMu.Unlock()
	defer func() {
		_ = out.Stop()
		_ = out.Close()
		a.txMu.Lock()
		if a.txID == id {
			a.audioOutput = nil
		}
		a.txMu.Unlock()
	}()
	if err := out.Open(ctx, audio.OutputConfig{DeviceID: as.OutputDeviceID, SampleRate: 12000, Channels: 1, BufferMS: as.BufferMS, Channel: as.OutputChannel, Level: as.TxAudioLevel}); err != nil {
		a.emitRaw("tx:error", map[string]any{"error": err.Error()})
		return
	}
	if err := a.setPTT(true); err != nil {
		a.emitRaw("tx:error", map[string]any{"error": err.Error()})
		return
	}
	a.emitRaw("tx:tune-state", map[string]any{"active": true, "frequencyHz": 1500})
	defer func() {
		a.txMu.Lock()
		active := a.txID == id
		a.txMu.Unlock()
		if active {
			_ = a.setPTT(false)
		}
	}()
	const sampleRate = 12000
	const durationSeconds = 60
	n := sampleRate * durationSeconds
	wave := make([]float32, n)
	phaseStep := 2 * math.Pi * float64(freq) / sampleRate
	phase := 0.0
	for i := range wave {
		wave[i] = float32(math.Sin(phase))
		phase += phaseStep
		if phase >= 2*math.Pi {
			phase -= 2 * math.Pi
		}
	}
	for {
		if err := out.Play(ctx, wave, sampleRate); err != nil {
			if ctx.Err() != nil {
				return
			}
			a.emitRaw("tx:error", map[string]any{"error": err.Error()})
			return
		}
	}
}
func (a *App) StopTransmit() {
	a.txMu.Lock()
	cancel := a.txCancel
	out := a.audioOutput
	active := cancel != nil
	a.txCancel = nil
	a.txKind = ""
	a.txMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if out != nil {
		_ = out.Stop()
	}
	_ = a.setPTT(false)
	if active {
		a.emitRaw("tx:reset", map[string]any{"reason": "stopped"})
	}
}

func (a *App) SetRXFrequency(hz int) error {
	if hz < 200 {
		hz = 200
	}
	if hz > 5000 {
		hz = 5000
	}
	a.frequencyMu.Lock()
	a.rxFrequency = hz
	a.frequencyMu.Unlock()
	a.settingsMu.Lock()
	a.settings.RXFrequencyHz = hz
	settings := a.settings
	a.settingsMu.Unlock()
	if a.configPath != "" {
		if err := storage.SaveSettings(a.configPath, settings); err != nil {
			a.emit(model.EventSettingsError, map[string]any{"error": err.Error()})
		}
	}
	a.receiverMu.Lock()
	stream := a.jttyStream
	a.receiverMu.Unlock()
	if stream != nil {
		stream.SetCenterFrequency(float64(hz))
	}
	a.emit(model.EventFrequencyRX, map[string]any{"frequencyHz": hz})
	return nil
}

func (a *App) SetTXFrequency(hz int) error {
	if hz < 200 {
		hz = 200
	}
	if hz > 5000 {
		hz = 5000
	}
	a.frequencyMu.Lock()
	a.txFrequency = hz
	a.frequencyMu.Unlock()
	a.settingsMu.Lock()
	a.settings.TXFrequencyHz = hz
	settings := a.settings
	a.settingsMu.Unlock()
	if a.configPath != "" {
		if err := storage.SaveSettings(a.configPath, settings); err != nil {
			a.emit(model.EventSettingsError, map[string]any{"error": err.Error()})
		}
	}
	a.emit(model.EventFrequencyTX, map[string]any{"frequencyHz": hz})
	return nil
}

func bandDialFrequency(band string) (int64, bool) {
	m := map[string]int64{
		"160": 1838000, "80": 3575000, "60": 5357000, "40": 7090000,
		"30": 10140000, "20": 14090000, "17": 18100000, "15": 21090000,
		"12": 24920000, "10": 28090000, "6": 50316000, "2": 144077000, "70": 432077000,
	}
	f, ok := m[strings.TrimSuffix(strings.TrimSpace(band), "m")]
	return f, ok
}

func normalizeFrequencyBands(s *model.Settings) bool {
	if s == nil {
		return false
	}
	changed := false
	defaults := model.DefaultFrequencyBands()
	byBand := make(map[string]model.FrequencyBandSettings, len(s.Frequencies))
	var customOrder []string
	seenBands := make(map[string]struct{}, len(s.Frequencies))
	for _, b := range s.Frequencies {
		band := strings.ToLower(strings.TrimSpace(b.Band))
		if band == "" {
			continue
		}
		if _, exists := seenBands[band]; exists {
			continue
		}
		seenBands[band] = struct{}{}
		if len(b.Frequencies) == 0 && b.DefaultHz > 0 {
			b.Frequencies = []int64{b.DefaultHz}
		}
		clean := make([]int64, 0, len(b.Frequencies))
		seen := make(map[int64]struct{}, len(b.Frequencies))
		for _, hz := range b.Frequencies {
			if hz <= 0 || hz > 2e10 {
				continue
			}
			if _, ok := seen[hz]; ok {
				continue
			}
			seen[hz] = struct{}{}
			clean = append(clean, hz)
		}
		b.Band = strings.TrimSpace(b.Band)
		b.Frequencies = clean
		if b.DefaultHz <= 0 || !containsFrequency(clean, b.DefaultHz) {
			if len(clean) > 0 {
				b.DefaultHz = clean[0]
			}
		}
		byBand[band] = b
		if !isDefaultFrequencyBand(band, defaults) {
			customOrder = append(customOrder, band)
		}
	}
	out := make([]model.FrequencyBandSettings, 0, len(defaults)+len(customOrder))
	for _, d := range defaults {
		key := strings.ToLower(d.Band)
		if b, ok := byBand[key]; ok {
			if len(b.Frequencies) == 0 {
				b.Frequencies = append([]int64(nil), d.Frequencies...)
				b.DefaultHz = d.DefaultHz
			}
			out = append(out, b)
			delete(byBand, key)
		} else {
			out = append(out, d)
			changed = true
		}
	}
	for _, key := range customOrder {
		if b, ok := byBand[key]; ok {
			out = append(out, b)
			delete(byBand, key)
		}
	}
	if len(out) != len(s.Frequencies) {
		changed = true
	}
	if !frequencyBandSlicesEqual(s.Frequencies, out) {
		changed = true
	}
	s.Frequencies = out
	return changed
}

func isDefaultFrequencyBand(key string, defaults []model.FrequencyBandSettings) bool {
	for _, d := range defaults {
		if strings.EqualFold(strings.TrimSpace(d.Band), key) {
			return true
		}
	}
	return false
}

func containsFrequency(values []int64, target int64) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func frequencyBandSlicesEqual(a, b []model.FrequencyBandSettings) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Band != b[i].Band || a[i].DefaultHz != b[i].DefaultHz || len(a[i].Frequencies) != len(b[i].Frequencies) {
			return false
		}
		for j := range a[i].Frequencies {
			if a[i].Frequencies[j] != b[i].Frequencies[j] {
				return false
			}
		}
	}
	return true
}

func bandDialFrequencyFromSettings(s model.Settings, band string) (int64, bool) {
	key := strings.ToLower(strings.TrimSpace(band))
	for _, b := range s.Frequencies {
		if strings.ToLower(strings.TrimSpace(b.Band)) != key {
			continue
		}
		if b.DefaultHz > 0 {
			return b.DefaultHz, true
		}
		if len(b.Frequencies) > 0 {
			return b.Frequencies[0], true
		}
	}
	return bandDialFrequency(band)
}

func radioControlMode(backend, pttMethod string) string {
	if strings.EqualFold(strings.TrimSpace(backend), "hamlib-rigctld") {
		return "HAMLIB / " + strings.ToUpper(strings.TrimSpace(pttMethod))
	}
	if strings.EqualFold(strings.TrimSpace(pttMethod), "VOX") || strings.TrimSpace(pttMethod) == "" {
		return "VOX / AUDIO"
	}
	return strings.ToUpper(strings.TrimSpace(pttMethod)) + " / AUDIO"
}

func (a *App) SetDialFrequency(hz int64) error {
	if hz < 0 {
		hz = 0
	}
	a.radioMu.Lock()
	r := a.radio
	a.radioMu.Unlock()
	if r == nil {
		a.frequencyMu.Lock()
		a.dialFrequency = hz
		a.frequencyMu.Unlock()
		a.settingsMu.RLock()
		backend := a.settings.Radio.Backend
		pttMethod := a.settings.Radio.PTTMethod
		mode := a.settings.Radio.Mode
		a.settingsMu.RUnlock()
		a.emit(model.EventRadioState, map[string]any{"connected": false, "frequencyHz": hz, "mode": mode, "controlMode": radioControlMode(backend, pttMethod)})
		return nil
	}
	if err := r.SetFrequency(context.Background(), hz); err != nil {
		return err
	}
	a.settingsMu.RLock()
	mode := a.settings.Radio.Mode
	a.settingsMu.RUnlock()
	if strings.TrimSpace(mode) != "" {
		if mr, ok := r.(interface {
			SetMode(context.Context, string) error
		}); ok {
			_ = mr.SetMode(context.Background(), mode)
		}
	}
	a.frequencyMu.Lock()
	a.dialFrequency = hz
	a.frequencyMu.Unlock()
	a.settingsMu.RLock()
	split := a.settings.Radio.SplitMode
	a.settingsMu.RUnlock()
	if split == "Rig" {
		if sr, ok := r.(interface {
			SetSplit(context.Context, string) error
			SetSplitFrequency(context.Context, int64) error
		}); ok {
			_ = sr.SetSplit(context.Background(), "VFOB")
			_ = sr.SetSplitFrequency(context.Background(), hz)
		}
	}
	a.settingsMu.RLock()
	backend := a.settings.Radio.Backend
	pttMethod := a.settings.Radio.PTTMethod
	modeState := a.settings.Radio.Mode
	a.settingsMu.RUnlock()
	a.emit(model.EventRadioState, map[string]any{"connected": true, "frequencyHz": hz, "mode": modeState, "controlMode": radioControlMode(backend, pttMethod)})
	return nil
}

func (a *App) Frequencies() map[string]int {
	a.frequencyMu.RLock()
	defer a.frequencyMu.RUnlock()
	return map[string]int{"rx": a.rxFrequency, "tx": a.txFrequency, "dial": int(a.dialFrequency)}
}

func (a *App) SetDXCall(call string) {
	c := jtty.NormalizeCallsign(call)
	a.qsoMu.Lock()
	a.dxCall = c
	a.qsoMu.Unlock()
	a.emit(model.EventDXCallChanged, model.DXCallChanged{Call: c})
}
func (a *App) SetDXGrid(grid string) {
	g := strings.ToUpper(strings.TrimSpace(grid))
	if !validGrid(g) {
		g = ""
	}
	a.qsoMu.Lock()
	a.dxGrid = g
	a.qsoMu.Unlock()
	a.emitRaw("qso:dxgrid", map[string]any{"grid": g})
}

func (a *App) DXGrid() string {
	a.qsoMu.Lock()
	g := a.dxGrid
	a.qsoMu.Unlock()
	return g
}

func validGrid(g string) bool {
	if len(g) != 4 && len(g) != 6 {
		return false
	}
	if g[0] < 'A' || g[0] > 'R' || g[1] < 'A' || g[1] > 'R' || g[2] < '0' || g[2] > '9' || g[3] < '0' || g[3] > '9' {
		return false
	}
	if len(g) == 6 && (g[4] < 'A' || g[4] > 'X' || g[5] < 'A' || g[5] > 'X') {
		return false
	}
	return true
}

func (a *App) DXCall() string { a.qsoMu.Lock(); defer a.qsoMu.Unlock(); return a.dxCall }

func (a *App) AddTestDecode(message string, frequencyHz, snr int, dt float64) model.DecodeMessage {
	now := time.Now().UTC()
	item := model.DecodeMessage{SignalUTC: now, ReceivedUTC: now, SNR: snr, DT: dt, FrequencyHz: frequencyHz, Message: message, Callsigns: jtty.ExtractCallsigns(message), Confidence: 1}
	a.settingsMu.RLock()
	stored := a.store
	a.settingsMu.RUnlock()
	item = stored.Append(item)
	a.sequence.Store(item.Sequence)
	a.emit(model.EventDecodeAdded, item)
	return item
}

func (a *App) DecodeHistory() []model.DecodeMessage {
	a.settingsMu.RLock()
	s := a.store
	a.settingsMu.RUnlock()
	return s.Snapshot()
}

func (a *App) RenderMacro(id, exchange, queueCall string) string {
	a.settingsMu.Lock()
	my := a.settings.MyCall
	grid := a.settings.MyGrid
	call := a.dxCall
	macros := append([]model.MacroConfig(nil), a.settings.Macros...)
	for _, m := range macros {
		if m.ID != id || !m.Enabled {
			continue
		}
		if strings.Contains(m.Template, "%E") {
			serial := a.settings.ExchangeSerialNumber
			if serial < 1 {
				serial = 1
			}
			exchange = fmt.Sprintf("599 %03d", serial)
			a.settings.ExchangeSerialNumber = serial + 1
			settings := a.settings
			configPath := a.configPath
			a.settingsMu.Unlock()
			if configPath != "" {
				if err := storage.SaveSettings(configPath, settings); err != nil {
					a.emit(model.EventSettingsError, map[string]any{"error": err.Error()})
					return ""
				}
			}
			a.emit(model.EventSettingsState, settings)
			a.emitRaw("qso:exchange", map[string]any{
				"exchange":         exchange,
				"serialNumber":     serial,
				"nextSerialNumber": serial + 1,
				"call":             call,
				"startUtc":         time.Now().UTC(),
			})
			return macro.Render(m.Template, macro.Context{MyCall: my, MyGrid: grid, DXCall: call, Exchange: exchange, QueueCall: queueCall})
		}
		result := macro.Render(m.Template, macro.Context{MyCall: my, MyGrid: grid, DXCall: call, Exchange: exchange, QueueCall: queueCall})
		a.settingsMu.Unlock()
		return result
	}
	a.settingsMu.Unlock()
	return ""
}

func (a *App) triggerAndTransmitMacro(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	now := time.Now()
	a.macroMu.Lock()
	duplicate := a.lastMacroID == id && now.Sub(a.lastMacroTrigger) < 300*time.Millisecond
	if !duplicate {
		a.lastMacroID = id
		a.lastMacroTrigger = now
	}
	a.macroMu.Unlock()
	if duplicate {
		return
	}
	message := a.TriggerMacro(id, "", "")
	if strings.TrimSpace(message) != "" {
		go a.transmitJTTY(id, message)
	}
}

func (a *App) ProtocolInfo() map[string]any {
	return map[string]any{"baud": jtty.CurrentProtocol.Baud, "tones": jtty.CurrentProtocol.Tones, "bandwidthHz": jtty.CurrentProtocol.BandwidthHz, "txDurationSeconds": jtty.CurrentProtocol.TxDurationSeconds}
}
func (a *App) Health() string { return fmt.Sprintf("JTTY-Go core OK (%s)", runtime.GOOS) }

func normalizeFrequencyMax(v float64) float64 {
	allowed := []float64{2500, 2700, 3000, 3500, 4000}
	best := allowed[1]
	delta := math.Abs(v - best)
	for _, candidate := range allowed {
		if math.Abs(v-candidate) < delta {
			best, delta = candidate, math.Abs(v-candidate)
		}
	}
	return best
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func numericFrequency64(v interface{}) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int64:
		return x, true
	case uint64:
		return int64(x), true
	case float64:
		return int64(x), true
	case float32:
		return int64(x), true
	case string:
		if n, err := strconv.ParseFloat(strings.TrimSpace(x), 64); err == nil {
			return int64(n), true
		}
	}
	return 0, false
}

func numericFrequency(v interface{}) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int8:
		return int(x), true
	case int16:
		return int(x), true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case uint:
		return int(x), true
	case uint8:
		return int(x), true
	case uint16:
		return int(x), true
	case uint32:
		return int(x), true
	case uint64:
		return int(x), true
	case float32:
		return int(x), true
	case float64:
		return int(x), true
	case map[string]interface{}:
		if y, ok := x["frequencyHz"]; ok {
			return numericFrequency(y)
		}
	}
	return 0, false
}
