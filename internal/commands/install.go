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
	"github.com/svngoku/hermes-cli/internal/ui"
)

type InstallState struct {
	SGLangInstalled   bool      `json:"sglang_installed"`
	SGLangVersion     string    `json:"sglang_version,omitempty"`
	VLLMInstalled     bool      `json:"vllm_installed"`
	VLLMVersion       string    `json:"vllm_version,omitempty"`
	LlamaCppInstalled bool      `json:"llamacpp_installed"`
	LlamaCppVersion   string    `json:"llamacpp_version,omitempty"`
	LastUpdated       time.Time `json:"last_updated"`
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
	installMode := fs.String("install", "both", "Install mode: sglang|vllm|llamacpp|both|all|none")
	check := fs.Bool("check", false, "Check installation status without changes")
	fs.Usage = func() {
		fmt.Fprintln(ctx.Stdout, "Usage: hermes install [flags]")
		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintln(ctx.Stdout, "Install or check inference engines")
		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintln(ctx.Stdout, "SGLang and vLLM install into dedicated virtual environments")
		fmt.Fprintln(ctx.Stdout, "(~/sglang-env, ~/vllm-env); llama.cpp is built from source and")
		fmt.Fprintln(ctx.Stdout, "installed into ~/.local/bin. All output streams live below.")
		fmt.Fprintln(ctx.Stdout)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	mode := config.InstallMode(*installMode)
	if mode != config.InstallSGLang && mode != config.InstallVLLM &&
		mode != config.InstallLlamaCpp && mode != config.InstallBoth &&
		mode != config.InstallAll && mode != config.InstallNone {
		return fmt.Errorf("invalid install mode: %s", *installMode)
	}

	fmt.Fprintln(ctx.Stdout, ui.Banner())
	fmt.Fprintln(ctx.Stdout, ui.Step("Installation check..."))
	fmt.Fprintln(ctx.Stdout, ui.HR())

	state, _ := loadState()
	engines := selectedEngines(mode)

	type engineStatus struct {
		engine    engine.Engine
		installed bool
		version   string
	}
	statuses := make([]engineStatus, 0, len(engines))
	for _, eng := range engines {
		installed, version, err := eng.CheckInstalled(ctx.Ctx)
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

	for _, status := range statuses {
		if status.installed {
			fmt.Fprintln(ctx.Stdout, ui.Ok(status.engine.Name()+" already installed"))
			continue
		}
		fmt.Fprintln(ctx.Stdout, ui.HR())
		fmt.Fprintln(ctx.Stdout, ui.Step("Installing "+status.engine.Name()+"..."))
		fmt.Fprintln(ctx.Stdout, ui.Info("Streaming live output; this can take several minutes"))
		if err := status.engine.Install(ctx.Ctx, ctx.Stdout, ctx.Stderr); err != nil {
			fmt.Fprintln(ctx.Stdout, ui.Fail(status.engine.Name()+" installation failed: "+err.Error()))
			return err
		}
		installed, version, err := status.engine.CheckInstalled(ctx.Ctx)
		if err != nil {
			return err
		}
		if !installed {
			return fmt.Errorf("%s installation completed but the engine is still unavailable", status.engine.Name())
		}
		updateInstallState(state, status.engine.Name(), installed, version)
		fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("%s installed: %s", status.engine.Name(), version)))
	}

	if err := saveState(state); err != nil {
		ctx.Logger.Warn("failed to save state", "error", err)
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
	case config.InstallAll:
		return []engine.Engine{
			engine.Get(config.EngineSGLang),
			engine.Get(config.EngineVLLM),
			engine.Get(config.EngineLlamaCpp),
		}
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
	case string(config.EngineLlamaCpp):
		state.LlamaCppInstalled = installed
		state.LlamaCppVersion = version
	}
}
