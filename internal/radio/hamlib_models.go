package radio

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// HamlibModel is a supported radio backend reported by rigctld/rigctl -l.
type HamlibModel struct {
	ModelID      int    `json:"modelId"`
	Label        string `json:"label"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Model        string `json:"model,omitempty"`
	Version      string `json:"version,omitempty"`
	Status       string `json:"status,omitempty"`
}

// HamlibCapabilities is the capability subset required by the WSJT-X-style
// configuration invariants. The values are obtained from Hamlib itself with
// `rigctl -m <model> -u`, never guessed from the display name.
type HamlibCapabilities struct {
	ModelID                 int      `json:"modelId"`
	PortType                string   `json:"portType"`
	PTTType                 string   `json:"pttType"`
	HasCATPTT               bool     `json:"hasCATPTT"`
	HasCATPTTMicData        bool     `json:"hasCATPTTMicData"`
	HasCATIndirectSerialPTT bool     `json:"hasCATIndirectSerialPTT"`
	Asynchronous            bool     `json:"asynchronous"`
	HasGetFreq              bool     `json:"hasGetFreq"`
	HasSetFreq              bool     `json:"hasSetFreq"`
	HasGetMode              bool     `json:"hasGetMode"`
	HasSetMode              bool     `json:"hasSetMode"`
	HasGetPTT               bool     `json:"hasGetPTT"`
	HasSetPTT               bool     `json:"hasSetPTT"`
	HasGetSplitVFO          bool     `json:"hasGetSplitVFO"`
	HasSetSplitVFO          bool     `json:"hasSetSplitVFO"`
	HasGetSplitFreq         bool     `json:"hasGetSplitFreq"`
	HasSetSplitFreq         bool     `json:"hasSetSplitFreq"`
	HasGetSplitMode         bool     `json:"hasGetSplitMode"`
	HasSetSplitMode         bool     `json:"hasSetSplitMode"`
	SupportedModes          []string `json:"supportedModes,omitempty"`
}

// ParseHamlibModelList parses the human-readable model table emitted by
// rigctld -l / rigctl -l. We intentionally retain the original row as Label
// because Hamlib model names can contain spaces and the column widths vary.
func ParseHamlibModelList(output string) []HamlibModel {
	var result []HamlibModel
	scanner := bufio.NewScanner(strings.NewReader(output))
	seen := make(map[int]struct{})

	// Hamlib's list is a fixed-column table. Keep the column positions from the
	// header so manufacturers/models containing spaces are parsed correctly
	// (for example: "N2ADR James Ahlstrom" + "Quisk").
	mfgStart, modelStart, versionStart, statusStart, macroStart := -1, -1, -1, -1, -1
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		upper := strings.ToUpper(trimmed)
		if strings.Contains(upper, "MFG") && strings.Contains(upper, "MODEL") && strings.Contains(upper, "VERSION") && strings.Contains(upper, "STATUS") && strings.Contains(upper, "MACRO") {
			if idx := strings.Index(strings.ToUpper(line), "MFG"); idx >= 0 {
				mfgStart = idx
			}
			if idx := strings.Index(strings.ToUpper(line), "MODEL"); idx >= 0 {
				modelStart = idx
			}
			if idx := strings.Index(strings.ToUpper(line), "VERSION"); idx >= 0 {
				versionStart = idx
			}
			if idx := strings.Index(strings.ToUpper(line), "STATUS"); idx >= 0 {
				statusStart = idx
			}
			if idx := strings.Index(strings.ToUpper(line), "MACRO"); idx >= 0 {
				macroStart = idx
			}
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		id, err := strconv.Atoi(fields[0])
		if err != nil || id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}

		manufacturer, model, version, status := "", "", "", ""
		if mfgStart >= 0 && modelStart > mfgStart && versionStart > modelStart && statusStart > versionStart {
			manufacturer = fixedColumn(line, mfgStart, modelStart)
			model = fixedColumn(line, modelStart, versionStart)
			version = fixedColumn(line, versionStart, statusStart)
			if macroStart > statusStart {
				status = fixedColumn(line, statusStart, macroStart)
			} else {
				status = strings.TrimSpace(strings.Join(fields[len(fields)-2:len(fields)-1], " "))
			}
		}

		// Fallback for older/custom Hamlib output without a parseable header.
		if model == "" {
			macro := -1
			for i := len(fields) - 1; i >= 2; i-- {
				if strings.HasPrefix(strings.ToUpper(fields[i]), "RIG_MODEL_") {
					macro = i
					break
				}
			}
			if macro >= 0 && macro-2 >= 2 {
				manufacturer = fields[1]
				model = strings.Join(fields[2:macro-2], " ")
				version = fields[macro-2]
				status = fields[macro-1]
			} else if len(fields) >= 4 {
				manufacturer = fields[1]
				model = strings.Join(fields[2:len(fields)-2], " ")
				version = fields[len(fields)-2]
				status = fields[len(fields)-1]
			}
		}

		manufacturer = strings.TrimSpace(manufacturer)
		model = strings.TrimSpace(model)
		version = strings.TrimSpace(version)
		status = strings.TrimSpace(status)
		if model == "" {
			continue
		}
		label := model
		if manufacturer != "" {
			label = manufacturer + " " + model
		}
		result = append(result, HamlibModel{ModelID: id, Label: label, Manufacturer: manufacturer, Model: model, Version: version, Status: status})
		seen[id] = struct{}{}
	}

	sort.SliceStable(result, func(i, j int) bool {
		li := strings.ToUpper(strings.TrimSpace(result[i].Label))
		lj := strings.ToUpper(strings.TrimSpace(result[j].Label))
		if li != lj {
			return li < lj
		}
		return result[i].ModelID < result[j].ModelID
	})
	return result
}

func fixedColumn(line string, start, end int) string {
	if start < 0 || end <= start || start >= len(line) {
		return ""
	}
	if end > len(line) {
		end = len(line)
	}
	return strings.TrimSpace(line[start:end])
}

// ParseHamlibCapabilities parses the human-readable output of
// `rigctl -m <model> -u`. Hamlib has kept this dump intentionally readable,
// but exact spacing/column alignment can change, so the parser keys off field
// names rather than fixed positions and fails closed for boolean capabilities.
func ParseHamlibCapabilities(output string, modelID int) HamlibCapabilities {
	c := HamlibCapabilities{ModelID: modelID, PortType: "none"}
	var modeBuf []string
	modeSeen := false
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		value := valueAfterLabel(line)
		vlower := strings.ToLower(value)

		switch {
		case strings.HasPrefix(lower, "port type"):
			c.PortType = normalizeHamlibPortType(value)
		case strings.HasPrefix(lower, "ptt type"):
			c.PTTType = strings.ToUpper(strings.TrimSpace(value))
		case strings.HasPrefix(lower, "transceive"):
			// RIG_TRN_RIG means unsolicited/async state updates are supported.
			c.Asynchronous = strings.Contains(strings.ToUpper(value), "RIG_TRN_RIG")
		case containsCapabilityName(lower, "set_freq"):
			if v, ok := parseYesNo(value); ok {
				c.HasSetFreq = v
			}
		case containsCapabilityName(lower, "get_freq"):
			if v, ok := parseYesNo(value); ok {
				c.HasGetFreq = v
			}
		case containsCapabilityName(lower, "set_mode"):
			if v, ok := parseYesNo(value); ok {
				c.HasSetMode = v
			}
		case containsCapabilityName(lower, "get_mode"):
			if v, ok := parseYesNo(value); ok {
				c.HasGetMode = v
			}
		case containsCapabilityName(lower, "set_ptt"):
			if v, ok := parseYesNo(value); ok {
				c.HasSetPTT = v
			}
		case containsCapabilityName(lower, "get_ptt"):
			if v, ok := parseYesNo(value); ok {
				c.HasGetPTT = v
			}
		case containsCapabilityName(lower, "set_split_vfo"):
			if v, ok := parseYesNo(value); ok {
				c.HasSetSplitVFO = v
			}
		case containsCapabilityName(lower, "get_split_vfo"):
			if v, ok := parseYesNo(value); ok {
				c.HasGetSplitVFO = v
			}
		case containsCapabilityName(lower, "set_split_freq"):
			if v, ok := parseYesNo(value); ok {
				c.HasSetSplitFreq = v
			}
		case containsCapabilityName(lower, "get_split_freq"):
			if v, ok := parseYesNo(value); ok {
				c.HasGetSplitFreq = v
			}
		case containsCapabilityName(lower, "set_split_mode"):
			if v, ok := parseYesNo(value); ok {
				c.HasSetSplitMode = v
			}
		case containsCapabilityName(lower, "get_split_mode"):
			if v, ok := parseYesNo(value); ok {
				c.HasGetSplitMode = v
			}
		case strings.Contains(lower, "ptt mic data") || strings.Contains(lower, "ptt_mic_data") || strings.Contains(lower, "set_ptt_mic") || strings.Contains(lower, "set_ptt_data"):
			if v, ok := parseYesNo(value); ok {
				c.HasCATPTTMicData = v
			}
		case strings.HasPrefix(lower, "mode list"):
			modeSeen = true
			modeBuf = append(modeBuf, strings.Fields(value)...)
		case modeSeen && strings.HasPrefix(lower, "\t"):
			modeBuf = append(modeBuf, strings.Fields(value)...)
		}
		_ = vlower
	}

	// Hamlib's canonical PTT type is the strongest pre-instantiation signal for
	// WSJT-X's has_CAT_PTT() capability. Require both a CAT PTT type and a
	// concrete setter so an incomplete backend remains disabled in the GUI.
	c.HasCATPTT = strings.Contains(c.PTTType, "RIG_PTT_RIG") && c.HasSetPTT

	// Hamlib does not expose the WSJT-X mic/data switch capability as a separate
	// standard dump_caps field on all backends. Where a backend does expose an
	// explicit field, honor it; otherwise use the CAT PTT setter as the same
	// conservative capability surface used by the rigctld integration.
	if !strings.Contains(strings.ToLower(output), "ptt mic data") &&
		!strings.Contains(strings.ToLower(output), "ptt_mic_data") &&
		!strings.Contains(strings.ToLower(output), "set_ptt_mic") &&
		!strings.Contains(strings.ToLower(output), "set_ptt_data") {
		c.HasCATPTTMicData = c.HasCATPTT
	}

	for _, token := range modeBuf {
		token = strings.TrimSpace(strings.Trim(token, ",;"))
		if token == "" {
			continue
		}
		token = strings.ToUpper(token)
		known := false
		for _, old := range c.SupportedModes {
			if old == token {
				known = true
				break
			}
		}
		if !known && token != "MODE" && token != "LIST:" {
			c.SupportedModes = append(c.SupportedModes, token)
		}
	}
	return c
}

func valueAfterLabel(line string) string {
	if i := strings.IndexAny(line, ":="); i >= 0 && i+1 < len(line) {
		return strings.TrimSpace(line[i+1:])
	}
	parts := strings.Fields(line)
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[1:], " ")
}

func normalizeHamlibPortType(value string) string {
	v := strings.ToUpper(strings.TrimSpace(value))
	switch {
	case strings.Contains(v, "RIG_PORT_SERIAL"):
		return "serial"
	case strings.Contains(v, "RIG_PORT_NETWORK"):
		return "network"
	case strings.Contains(v, "RIG_PORT_USB"):
		return "usb"
	case strings.Contains(v, "TCI"):
		return "tci"
	default:
		return "none"
	}
}

func containsCapabilityName(line, name string) bool {
	return strings.Contains(line, "can "+name) || strings.Contains(line, "can"+name) || strings.Contains(line, name+":")
}

func parseYesNo(value string) (bool, bool) {
	v := strings.ToLower(strings.TrimSpace(value))
	v = strings.TrimSpace(strings.Trim(v, "[]()"))
	if strings.HasPrefix(v, "y") || strings.HasPrefix(v, "yes") || strings.HasPrefix(v, "true") || strings.HasPrefix(v, "1 ") || v == "1" {
		return true, true
	}
	if strings.HasPrefix(v, "n") || strings.HasPrefix(v, "no") || strings.HasPrefix(v, "false") || strings.HasPrefix(v, "0 ") || v == "0" {
		return false, true
	}
	return false, false
}

func hamlibRigctlCandidates(executable string) []string {
	exe := strings.TrimSpace(executable)
	if exe == "" {
		return nil
	}
	resolved, err := filepath.Abs(exe)
	if err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	base := strings.ToLower(filepath.Base(exe))
	candidates := make([]string, 0, 3)
	if base == "rigctl.exe" || base == "rigctl" {
		candidates = append(candidates, exe)
	} else {
		for _, name := range []string{"rigctl.exe", "rigctl"} {
			candidate := filepath.Join(dir, name)
			if _, err := os.Stat(candidate); err == nil {
				candidates = append(candidates, candidate)
			}
		}
		candidates = append(candidates, exe)
	}
	return candidates
}

// HamlibModelCapabilities executes Hamlib's model-only capability dump. It does
// not open the physical radio, so capability-driven enable/disable decisions
// are deterministic before a CAT connection exists.
func HamlibModelCapabilities(ctx context.Context, executable string, modelID int) (HamlibCapabilities, error) {
	if modelID <= 0 {
		return HamlibCapabilities{ModelID: 0, PortType: "none"}, nil
	}
	candidates := hamlibRigctlCandidates(resolveHamlibExecutable(executable))
	if len(candidates) == 0 {
		return HamlibCapabilities{}, fmt.Errorf("未找到 rigctl.exe/rigctld.exe")
	}
	var lastErr error
	for _, candidate := range candidates {
		cmd := exec.CommandContext(ctx, candidate, "-m", strconv.Itoa(modelID), "-u")
		cmd.Dir = filepath.Dir(candidate)
		cmd.Env = prependPath(cmd.Environ(), cmd.Dir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			lastErr = fmt.Errorf("%s: %w: %s", filepath.Base(candidate), err, strings.TrimSpace(string(out)))
			continue
		}
		caps := ParseHamlibCapabilities(string(out), modelID)
		if caps.PortType == "none" && caps.PTTType == "" && !caps.HasGetFreq && !caps.HasSetFreq && len(caps.SupportedModes) == 0 {
			lastErr = fmt.Errorf("%s 未返回可解析的 Hamlib capability", filepath.Base(candidate))
			continue
		}
		return caps, nil
	}
	if lastErr != nil {
		return HamlibCapabilities{}, fmt.Errorf("读取 Hamlib 设备能力失败: %w", lastErr)
	}
	return HamlibCapabilities{}, fmt.Errorf("读取 Hamlib 设备能力失败")
}

// ListHamlibModels executes rigctld/rigctl -l and returns supported models.
func ListHamlibModels(ctx context.Context, executable string) ([]HamlibModel, error) {
	exe := resolveHamlibExecutable(executable)
	if exe == "" {
		return nil, fmt.Errorf("未找到 rigctld.exe/rigctl.exe，请检查内置 Hamlib")
	}
	dir := filepath.Dir(exe)
	// WSJT-X gets its model registry directly from Hamlib. In the JTTY
	// rigctld architecture we use the lightweight -l command as the registry
	// source and prefer rigctl.exe for enumeration because it has no daemon
	// startup path to fail. Fall back to the selected executable when rigctl is
	// not present, keeping both programs in the same bundled directory.
	candidates := []string{exe}
	if filepath.Base(exe) != "rigctl.exe" && filepath.Base(exe) != "rigctl" {
		for _, name := range []string{"rigctl.exe", "rigctl"} {
			candidate := filepath.Join(dir, name)
			if _, err := os.Stat(candidate); err == nil {
				candidates = append([]string{candidate}, candidates...)
				break
			}
		}
	}
	var lastErr error
	for _, candidate := range candidates {
		cmd := exec.CommandContext(ctx, candidate, "-l")
		cmd.Dir = dir
		cmd.Env = prependPath(cmd.Environ(), dir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			lastErr = fmt.Errorf("%s: %w: %s", filepath.Base(candidate), err, strings.TrimSpace(string(out)))
			continue
		}
		models := ParseHamlibModelList(string(out))
		if len(models) > 0 {
			return models, nil
		}
		lastErr = fmt.Errorf("%s 未返回可用电台列表", filepath.Base(candidate))
	}
	if lastErr != nil {
		return nil, fmt.Errorf("读取 Hamlib 电台列表失败: %w", lastErr)
	}
	return nil, fmt.Errorf("读取 Hamlib 电台列表失败")
}
