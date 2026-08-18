package settingsstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/svngoku/hermes-cli/internal/config"
)

func TestSaveAndLoadWithProjectOverride(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)
	chdir(t, project)

	if _, err := Save(config.Settings{
		Engine: config.EngineSGLang,
		Model:  "user/model",
		TP:     4,
		Host:   "127.0.0.1",
		Port:   30000,
	}, "user"); err != nil {
		t.Fatal(err)
	}
	projectPath, err := Save(config.Settings{
		Engine: config.EngineLlamaCpp,
		HFRepo: "owner/repository:Q4_K_M",
		TP:     1,
		Port:   8080,
	}, "project")
	if err != nil {
		t.Fatal(err)
	}

	settings, found, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !found || settings.Engine != config.EngineLlamaCpp || settings.Model != "" {
		t.Errorf("settings = %#v, found = %t", settings, found)
	}
	info, err := os.Stat(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("config mode = %o, want 600", info.Mode().Perm())
	}
}

func TestLoadRejectsInvalidAndNonRegularProjectConfig(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		project := t.TempDir()
		t.Setenv("HOME", t.TempDir())
		chdir(t, project)
		if err := os.WriteFile(filepath.Join(project, ProjectConfigName), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := Load(); err == nil {
			t.Fatal("Load() error = nil for invalid JSON")
		}
	})

	t.Run("symlink", func(t *testing.T) {
		project := t.TempDir()
		t.Setenv("HOME", t.TempDir())
		chdir(t, project)
		target := filepath.Join(project, "target")
		if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(project, ProjectConfigName)); err != nil {
			t.Fatal(err)
		}
		if _, _, err := Load(); err == nil {
			t.Fatal("Load() error = nil for symlink")
		}
	})
}

func TestSaveReplacesSymlinkWithoutFollowingIt(t *testing.T) {
	project := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	chdir(t, project)
	target := filepath.Join(project, "target")
	if err := os.WriteFile(target, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(project, ProjectConfigName)
	if err := os.Symlink(target, configPath); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(config.Settings{Engine: config.EngineVLLM}, "project"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "unchanged" {
		t.Errorf("symlink target = %q", data)
	}
}

func chdir(t *testing.T, path string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previous)
	})
}
