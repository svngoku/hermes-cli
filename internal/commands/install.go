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
	installMode := fs.String("install", "both", "Install mode: sglang|vllm|both|none")
	check := fs.Bool("check", false, "Check installation status without changes")
	venvDir := fs.String("venv", ".venv", "Virtual environment directory")
	fs.Usage = func() {
		fmt.Fprintln(ctx.Stdout, "Usage: hermes install [flags]")
		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintln(ctx.Stdout, "Install inference engines (sglang, vllm)")
		fmt.Fprintln(ctx.Stdout)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	mode := config.InstallMode(*installMode)
	if mode != config.InstallSGLang && mode != config.InstallVLLM &&
		mode != config.InstallBoth && mode != config.InstallNone {
		return fmt.Errorf("invalid install mode: %s", *installMode)
	}

	fmt.Fprintln(ctx.Stdout, ui.Banner())
	fmt.Fprintln(ctx.Stdout, ui.Step("Installation check..."))
	fmt.Fprintln(ctx.Stdout, ui.HR())

	state, _ := loadState()

	if err := ensureUV(ctx, state); err != nil {
		return err
	}

	if !*check && mode != config.InstallNone {
		if err := setupVenv(ctx, *venvDir, state); err != nil {
			return err
		}
	}

	sglangEngine := engine.Get(config.EngineSGLang)
	vllmEngine := engine.Get(config.EngineVLLM)

	installed, version, _ := sglangEngine.CheckInstalled(ctx.Ctx)
	state.SGLangInstalled = installed
	state.SGLangVersion = version
	if installed {
		fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("sglang: %s", version)))
	} else {
		fmt.Fprintln(ctx.Stdout, ui.Warn("sglang: not installed"))
	}

	installed, version, _ = vllmEngine.CheckInstalled(ctx.Ctx)
	state.VLLMInstalled = installed
	state.VLLMVersion = version
	if installed {
		fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("vllm: %s", version)))
	} else {
		fmt.Fprintln(ctx.Stdout, ui.Warn("vllm: not installed"))
	}

	if *check {
		fmt.Fprintln(ctx.Stdout, ui.HR())
		fmt.Fprintln(ctx.Stdout, ui.Info("Check mode - no changes made"))
		return nil
	}

	if mode == config.InstallNone {
		fmt.Fprintln(ctx.Stdout, ui.HR())
		fmt.Fprintln(ctx.Stdout, ui.Info("Install mode: none - skipping engine installation"))
		return nil
	}

	fmt.Fprintln(ctx.Stdout, ui.HR())
	fmt.Fprintln(ctx.Stdout, ui.Step("Installing engines..."))

	if mode == config.InstallSGLang || mode == config.InstallBoth {
		if !state.SGLangInstalled {
			fmt.Fprintln(ctx.Stdout, ui.Info("Installing sglang..."))
			if err := sglangEngine.Install(ctx.Ctx); err != nil {
				fmt.Fprintln(ctx.Stdout, ui.Fail("sglang installation failed: "+err.Error()))
				return err
			}
			installed, version, _ := sglangEngine.CheckInstalled(ctx.Ctx)
			state.SGLangInstalled = installed
			state.SGLangVersion = version
			fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("sglang installed: %s", version)))
		} else {
			fmt.Fprintln(ctx.Stdout, ui.Ok("sglang already installed"))
		}
	}

	if mode == config.InstallVLLM || mode == config.InstallBoth {
		if !state.VLLMInstalled {
			fmt.Fprintln(ctx.Stdout, ui.Info("Installing vllm..."))
			if err := vllmEngine.Install(ctx.Ctx); err != nil {
				fmt.Fprintln(ctx.Stdout, ui.Fail("vllm installation failed: "+err.Error()))
				return err
			}
			installed, version, _ := vllmEngine.CheckInstalled(ctx.Ctx)
			state.VLLMInstalled = installed
			state.VLLMVersion = version
			fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("vllm installed: %s", version)))
		} else {
			fmt.Fprintln(ctx.Stdout, ui.Ok("vllm already installed"))
		}
	}

	if err := saveState(state); err != nil {
		ctx.Logger.Warn("failed to save state", "error", err)
	}

	fmt.Fprintln(ctx.Stdout, ui.HR())
	fmt.Fprintln(ctx.Stdout, ui.Ok("Installation complete"))

	return nil
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
