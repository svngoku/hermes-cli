package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/svngoku/hermes-cli/internal/app"
	"github.com/svngoku/hermes-cli/internal/config"
	"github.com/svngoku/hermes-cli/internal/engine"
	"github.com/svngoku/hermes-cli/internal/execx"
	"github.com/svngoku/hermes-cli/internal/ui"
)

type InstallState struct {
	SGLangInstalled bool      `json:"sglang_installed"`
	SGLangVersion   string    `json:"sglang_version,omitempty"`
	VLLMInstalled   bool      `json:"vllm_installed"`
	VLLMVersion     string    `json:"vllm_version,omitempty"`
	UVInstalled     bool      `json:"uv_installed"`
	VenvPath        string    `json:"venv_path,omitempty"`
	LastUpdated     time.Time `json:"last_updated"`
}

func getStateFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "hermes", "state.json")
}

func loadState() (*InstallState, error) {
	path := getStateFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &InstallState{}, nil
		}
		return nil, err
	}
	var state InstallState
	if err := json.Unmarshal(data, &state); err != nil {
		return &InstallState{}, nil
	}
	return &state, nil
}

func saveState(state *InstallState) error {
	path := getStateFilePath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	state.LastUpdated = time.Now()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func Install(ctx *app.AppContext, args []string) error {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	installMode := fs.String("install", "both", "Install mode: sglang|vllm|llamacpp|both|none")
	check := fs.Bool("check", false, "Check installation status without changes")
	venvDir := fs.String("venv", ".venv", "Virtual environment directory")
	fs.Usage = func() {
		fmt.Fprintln(ctx.Stdout, "Usage: hermes install [flags]")
		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintln(ctx.Stdout, "Install or check inference engines")
		fmt.Fprintln(ctx.Stdout)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	mode := config.InstallMode(*installMode)
	if mode != config.InstallSGLang && mode != config.InstallVLLM &&
		mode != config.InstallLlamaCpp && mode != config.InstallBoth && mode != config.InstallNone {
		return fmt.Errorf("invalid install mode: %s", *installMode)
	}

	fmt.Fprintln(ctx.Stdout, ui.Banner())
	fmt.Fprintln(ctx.Stdout, ui.Step("Installation check..."))
	fmt.Fprintln(ctx.Stdout, ui.HR())

	state, _ := loadState()
	engines := selectedEngines(mode)
	absVenvPath, err := filepath.Abs(*venvDir)
	if err != nil {
		return fmt.Errorf("resolve venv path: %w", err)
	}

	if !*check {
		for _, eng := range engines {
			if eng.Profile().Kind == engine.RuntimeUVPython {
				if err := ensureUV(ctx, state); err != nil {
					return err
				}
				if err := setupVenv(ctx, *venvDir, state); err != nil {
					return err
				}
				break
			}
		}
	}

	type engineStatus struct {
		engine    engine.Engine
		installed bool
		version   string
	}
	statuses := make([]engineStatus, 0, len(engines))
	for _, eng := range engines {
		venvPath := ""
		if eng.Profile().Kind == engine.RuntimeUVPython {
			venvPath = absVenvPath
		}
		installed, version, err := engine.CheckInstalledIn(ctx.Ctx, eng, venvPath)
		if err != nil {
			return err
		}
		updateInstallState(state, eng.Name(), installed, version)
		statuses = append(statuses, engineStatus{engine: eng, installed: installed, version: version})
		if installed {
			fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("%s: %s", eng.Name(), version)))
		} else {
			fmt.Fprintln(ctx.Stdout, ui.Warn(fmt.Sprintf("%s: not installed", eng.Name())))
		}
	}

	if *check {
		fmt.Fprintln(ctx.Stdout, ui.HR())
		fmt.Fprintln(ctx.Stdout, ui.Info("Check mode - no changes made"))
		return nil
	}

	if len(engines) == 0 {
		fmt.Fprintln(ctx.Stdout, ui.HR())
		fmt.Fprintln(ctx.Stdout, ui.Info("Install mode: none - skipping engine installation"))
		return nil
	}

	fmt.Fprintln(ctx.Stdout, ui.HR())
	fmt.Fprintln(ctx.Stdout, ui.Step("Installing engines..."))

	for _, status := range statuses {
		if !status.installed {
			fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Installing %s...", status.engine.Name())))
			venvPath := ""
			if status.engine.Profile().Kind == engine.RuntimeUVPython {
				venvPath = absVenvPath
			}
			if err := engine.InstallIn(ctx.Ctx, status.engine, venvPath); err != nil {
				fmt.Fprintln(ctx.Stdout, ui.Fail(status.engine.Name()+" installation failed: "+err.Error()))
				return err
			}
			installed, version, err := engine.CheckInstalledIn(ctx.Ctx, status.engine, venvPath)
			if err != nil {
				return err
			}
			if !installed {
				return fmt.Errorf("%s installation completed but the engine is still unavailable", status.engine.Name())
			}
			updateInstallState(state, status.engine.Name(), installed, version)
			fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("%s installed: %s", status.engine.Name(), version)))
		} else {
			fmt.Fprintln(ctx.Stdout, ui.Ok(status.engine.Name()+" already installed"))
		}
	}

	if mode != config.InstallLlamaCpp {
		if err := saveState(state); err != nil {
			ctx.Logger.Warn("failed to save state", "error", err)
		}
	}

	fmt.Fprintln(ctx.Stdout, ui.HR())
	fmt.Fprintln(ctx.Stdout, ui.Ok("Installation complete"))

	return nil
}

func selectedEngines(mode config.InstallMode) []engine.Engine {
	switch mode {
	case config.InstallSGLang:
		return []engine.Engine{engine.Get(config.EngineSGLang)}
	case config.InstallVLLM:
		return []engine.Engine{engine.Get(config.EngineVLLM)}
	case config.InstallLlamaCpp:
		return []engine.Engine{engine.Get(config.EngineLlamaCpp)}
	case config.InstallBoth:
		return []engine.Engine{engine.Get(config.EngineSGLang), engine.Get(config.EngineVLLM)}
	default:
		return nil
	}
}

func updateInstallState(state *InstallState, name string, installed bool, version string) {
	switch name {
	case string(config.EngineSGLang):
		state.SGLangInstalled = installed
		state.SGLangVersion = version
	case string(config.EngineVLLM):
		state.VLLMInstalled = installed
		state.VLLMVersion = version
	}
}

func ensureUV(ctx *app.AppContext, state *InstallState) error {
	if execx.CommandExists("uv") {
		state.UVInstalled = true
		return nil
	}

	fmt.Fprintln(ctx.Stdout, ui.Info("Installing uv..."))
	result := execx.Run(ctx.Ctx, "sh", "-c", "curl -LsSf https://astral.sh/uv/install.sh | sh")
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to install uv: %s", result.Stderr)
	}

	os.Setenv("PATH", os.Getenv("HOME")+"/.local/bin:"+os.Getenv("PATH"))

	if execx.CommandExists("uv") {
		state.UVInstalled = true
		fmt.Fprintln(ctx.Stdout, ui.Ok("uv installed"))
		return nil
	}

	return fmt.Errorf("uv installation failed - command not found after install")
}

func setupVenv(ctx *app.AppContext, venvDir string, state *InstallState) error {
	absPath, _ := filepath.Abs(venvDir)

	if _, err := os.Stat(venvDir); err == nil {
		fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("venv exists: %s", absPath)))
		state.VenvPath = absPath
		return nil
	}

	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Creating venv: %s", absPath)))
	result := execx.Run(ctx.Ctx, "uv", "venv", venvDir)
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to create venv: %s", result.Stderr)
	}

	state.VenvPath = absPath
	fmt.Fprintln(ctx.Stdout, ui.Ok("venv created"))
	return nil
}
