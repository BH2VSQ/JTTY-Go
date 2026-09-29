package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/BH2VSQ/jtty-go/internal/model"
)

func DefaultSettings() model.Settings { return model.DefaultSettings() }

func SaveSettings(path string, settings model.Settings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadSettings(path string) (model.Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			settings := model.DefaultSettings()
			if saveErr := SaveSettings(path, settings); saveErr != nil {
				return settings, saveErr
			}
			return settings, nil
		}
		return model.Settings{}, err
	}
	settings := model.DefaultSettings()
	if err := json.Unmarshal(data, &settings); err != nil {
		return model.Settings{}, err
	}
	return settings, nil
}
