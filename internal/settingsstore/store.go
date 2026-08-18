package settingsstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/svngoku/hermes-cli/internal/config"
)

const (
	ProjectConfigName = ".hermes.json"
	maxConfigSize     = 1 << 20
)

func UserPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory: %w", err)
	}
	return filepath.Join(dir, "hermes", "config.json"), nil
}

func ProjectPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locate project directory: %w", err)
	}
	return filepath.Join(dir, ProjectConfigName), nil
}

func Load() (config.Settings, bool, error) {
	var merged config.Settings
	found := false

	userPath, err := UserPath()
	if err != nil {
		return config.Settings{}, false, err
	}
	projectPath, err := ProjectPath()
	if err != nil {
		return config.Settings{}, false, err
	}
	for _, path := range []string{userPath, projectPath} {
		settings, exists, err := read(path)
		if err != nil {
			return config.Settings{}, false, err
		}
		if exists {
			config.MergeSettings(&merged, settings)
			found = true
		}
	}
	return merged, found, nil
}

func Save(settings config.Settings, scope string) (string, error) {
	var path string
	var err error
	switch scope {
	case "user":
		path, err = UserPath()
	case "project":
		path, err = ProjectPath()
	default:
		return "", fmt.Errorf("invalid config scope %q (use user or project)", scope)
	}
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), ".hermes-config-*")
	if err != nil {
		return "", fmt.Errorf("create temporary config: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", fmt.Errorf("secure temporary config: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write temporary config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", fmt.Errorf("replace config: %w", err)
	}
	return path, nil
}

func read(path string) (config.Settings, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return config.Settings{}, false, nil
	}
	if err != nil {
		return config.Settings{}, false, fmt.Errorf("inspect config %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return config.Settings{}, false, fmt.Errorf("config %s must be a regular file", path)
	}
	if info.Size() > maxConfigSize {
		return config.Settings{}, false, fmt.Errorf("config %s exceeds %d bytes", path, maxConfigSize)
	}

	file, err := os.Open(path)
	if err != nil {
		return config.Settings{}, false, fmt.Errorf("read config %s: %w", path, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxConfigSize+1))
	if err != nil {
		return config.Settings{}, false, fmt.Errorf("read config %s: %w", path, err)
	}
	if len(data) > maxConfigSize {
		return config.Settings{}, false, fmt.Errorf("config %s exceeds %d bytes", path, maxConfigSize)
	}
	var settings config.Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return config.Settings{}, false, fmt.Errorf("parse config %s: %w", path, err)
	}
	return settings, true, nil
}
