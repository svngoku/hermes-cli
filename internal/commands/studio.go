package commands

import (
	"flag"
	"fmt"

	"github.com/svngoku/hermes-cli/internal/app"
	"github.com/svngoku/hermes-cli/internal/execx"
	"github.com/svngoku/hermes-cli/internal/ui"
)

func Studio(ctx *app.AppContext, args []string) error {
	fs := flag.NewFlagSet("studio", flag.ExitOnError)
	studioPort := fs.Int("studio-port", 8000, "Studio controller port")
	frontend := fs.Bool("frontend", false, "Launch frontend as well")
	check := fs.Bool("check", false, "Check if vllm-studio is installed")
	fs.Usage = func() {
		fmt.Fprintln(ctx.Stdout, "Usage: hermes studio [flags]")
		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintln(ctx.Stdout, "Launch vllm-studio controller")
		fmt.Fprintln(ctx.Stdout)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Fprintln(ctx.Stdout, ui.Banner())
	fmt.Fprintln(ctx.Stdout, ui.Step("vLLM Studio"))
	fmt.Fprintln(ctx.Stdout, ui.HR())

	result := execx.Run(ctx.Ctx, "python", "-c", "import vllm_studio; print(vllm_studio.__version__)")
	installed := result.ExitCode == 0

	if *check {
		if installed {
			fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("vllm-studio installed: %s", result.Stdout)))
		} else {
			fmt.Fprintln(ctx.Stdout, ui.Warn("vllm-studio not installed"))
			fmt.Fprintln(ctx.Stdout)
			fmt.Fprintln(ctx.Stdout, ui.Info("To install vllm-studio:"))
			fmt.Fprintln(ctx.Stdout, "  pip install git+https://github.com/0xSero/vllm-studio.git")
		}
		return nil
	}

	if !installed {
		fmt.Fprintln(ctx.Stdout, ui.Warn("vllm-studio not found"))
		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintln(ctx.Stdout, ui.Info("To install vllm-studio:"))
		fmt.Fprintln(ctx.Stdout, "  pip install git+https://github.com/0xSero/vllm-studio.git")
		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintln(ctx.Stdout, ui.Info("Then run:"))
		fmt.Fprintln(ctx.Stdout, fmt.Sprintf("  vllm-studio --port %d", *studioPort))
		if *frontend {
			fmt.Fprintln(ctx.Stdout)
			fmt.Fprintln(ctx.Stdout, ui.Info("For frontend (separate terminal):"))
			fmt.Fprintln(ctx.Stdout, "  cd vllm-studio/frontend && npm install && npm run dev")
		}
		return nil
	}

	fmt.Fprintln(ctx.Stdout, ui.Ok("vllm-studio found"))
	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Starting controller on port %d", *studioPort)))

	if *frontend {
		fmt.Fprintln(ctx.Stdout, ui.Info("For frontend (separate terminal):"))
		fmt.Fprintln(ctx.Stdout, "  cd vllm-studio/frontend && npm install && npm run dev")
	}

	fmt.Fprintln(ctx.Stdout, ui.HR())

	return execx.RunWithStreaming(
		ctx.Ctx,
		ctx.Stdout,
		ctx.Stderr,
		"vllm-studio",
		"--port", fmt.Sprintf("%d", *studioPort),
	)
}
