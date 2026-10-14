package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BH2VSQ/jtty-go/internal/model"
	"github.com/BH2VSQ/jtty-go/internal/radio"
)

func (a *App) stopRadioAudio() {
	a.txMu.Lock()
	active := a.txCancel != nil
	a.txMu.Unlock()
	if active {
		a.StopTransmit()
	}
}

func (a *App) openConfiguredRadio(cfg model.RadioSettings) (radio.Radio, radio.HamlibCapabilities, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	rc := radio.RigctldConfig{Address: cfg.Host, Port: cfg.Port, Mode: cfg.Mode, PassbandHz: cfg.PassbandHz}
	var r radio.Radio
	var caps radio.HamlibCapabilities
	if cfg.Backend == "hamlib-local" {
		if cfg.RigModelID <= 0 {
			return nil, caps, fmt.Errorf("请选择 Hamlib 电台型号")
		}
		executable := a.resolveRigctldForConfig(cfg)
		if executable == "" {
			return nil, caps, fmt.Errorf("未找到完整 Hamlib 运行包")
		}
		if err := radio.VerifyRuntime(filepath.Dir(executable)); err != nil {
			return nil, caps, err
		}
		var err error
		caps, err = radio.HamlibModelCapabilities(ctx, executable, cfg.RigModelID)
		if err != nil {
			return nil, caps, err
		}
		device := strings.TrimSpace(cfg.SerialPort)
		if strings.EqualFold(cfg.Handshake, "Hardware") && strings.EqualFold(cfg.PTTMethod, "RTS") && (cfg.PTTSerialPort == "" || strings.EqualFold(cfg.PTTSerialPort, device)) {
			return nil, caps, fmt.Errorf("硬件握手与 RTS PTT 不能共用同一串口，请选择独立 PTT 串口或其他方式")
		}
		if caps.PortType != "none" && device == "" {
			return nil, caps, fmt.Errorf("请填写电台设备路径或网络设备地址")
		}
		var args []string
		if caps.PortType == "serial" {
			args = buildRigctldArgs(cfg, "127.0.0.1", 0, device)
			// StartManaged chooses its own private endpoint.
			filtered := []string{}
			for i := 0; i < len(args); i++ {
				if args[i] == "-T" || args[i] == "-t" {
					i++
					continue
				}
				filtered = append(filtered, args[i])
			}
			args = filtered
		} else {
			args = []string{"-m", fmt.Sprint(cfg.RigModelID)}
			if device != "" {
				args = append(args, "-r", device)
			}
			method := strings.ToUpper(cfg.PTTMethod)
			if method == "RIGCTL" || method == "CAT" {
				method = "RIG"
			}
			if method == "VOX" || method == "" {
				method = "NONE"
			}
			args = append(args, "-P", method)
			if cfg.PTTSerialPort != "" {
				args = append(args, "-p", cfg.PTTSerialPort)
			}
		}
		managed, err := radio.StartManaged(ctx, executable, args, rc)
		if err != nil {
			return nil, caps, err
		}
		r = managed
	} else if cfg.Backend == "rigctld-net" {
		if strings.TrimSpace(cfg.Host) == "" || cfg.Port < 1 || cfg.Port > 65535 {
			return nil, caps, fmt.Errorf("NET rigctl 地址或端口无效")
		}
		netRadio := radio.NewRigctld(rc)
		if err := netRadio.Open(ctx); err != nil {
			return nil, caps, err
		}
		r = netRadio
	} else {
		return nil, caps, fmt.Errorf("尚未启用电台控制")
	}

	if probe, ok := r.(interface {
		Capabilities(context.Context) (radio.HamlibCapabilities, error)
	}); ok {
		actual, err := probe.Capabilities(ctx)
		if err != nil {
			_ = r.Close()
			return nil, caps, fmt.Errorf("读取电台能力: %w", err)
		}
		// Actual dump_caps describes the connected rig, not the NET model registry.
		actual.ModelID = cfg.RigModelID
		caps = actual
	}
	source := strings.ToUpper(cfg.TXAudioSource)
	pttValue := 1
	if source == "DATA" || source == "REAR" || source == "MIC" {
		if !caps.HasCATPTTMicData && (cfg.PTTMethod == "Rigctl" || cfg.PTTMethod == "CAT") {
			_ = r.Close()
			return nil, caps, fmt.Errorf("电台未明确支持麦克风/数据 PTT 选择，请选择默认音频输入")
		}
		if source == "MIC" {
			pttValue = 2
		} else {
			pttValue = 3
		}
	}
	if setter, ok := r.(interface{ SetPTTValue(int) }); ok {
		setter.SetPTTValue(pttValue)
	}
	if _, err := r.Frequency(ctx); err != nil {
		_ = r.Close()
		return nil, caps, fmt.Errorf("读取频率: %w", err)
	}
	return r, caps, nil
}

func configureSplit(ctx context.Context, r radio.Radio, on bool, hz int64) error {
	split, ok := r.(interface {
		Split(context.Context) (bool, string, error)
		SetSplitEnabled(context.Context, bool, string) error
		SetSplitFrequency(context.Context, int64) error
		CurrentVFO(context.Context) (string, error)
		Capabilities(context.Context) (radio.HamlibCapabilities, error)
	})
	if !ok {
		return fmt.Errorf("后端不支持 Split")
	}
	_, vfo, err := split.Split(ctx)
	if err != nil {
		return err
	}
	if vfo == "None" || vfo == "" {
		current, e := split.CurrentVFO(ctx)
		if e != nil {
			return e
		}
		vfo = current
		if on {
			caps, e := split.Capabilities(ctx)
			if e != nil {
				return e
			}
			vfo = ""
			for _, candidate := range caps.SupportedVFOs {
				if candidate != current && candidate != "MEM" && candidate != "currVFO" {
					vfo = candidate
					break
				}
			}
			if vfo == "" {
				return fmt.Errorf("未找到可用的 TX VFO")
			}
		}
	}
	if err = split.SetSplitEnabled(ctx, on, vfo); err != nil {
		return err
	}
	if on {
		return split.SetSplitFrequency(ctx, hz)
	}
	return nil
}

func (a *App) hamlibRuntimeCandidates() []string {
	exe, _ := os.Executable()
	paths := []string{filepath.Join(filepath.Dir(exe), "hamlib", "rigctld.exe")}
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, "bin", "rigctld.exe"))
	}
	return paths
}
