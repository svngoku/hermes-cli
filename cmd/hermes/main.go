package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/svngoku/hermes-cli/internal/app"
	"github.com/svngoku/hermes-cli/internal/commands"
	"github.com/svngoku/hermes-cli/internal/ui"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]

	if cmd == "-h" || cmd == "--help" || cmd == "help" {
		printUsage()
		os.Exit(0)
	}

	if cmd == "version" || cmd == "--version" || cmd == "-v" {
		printVersion()
		os.Exit(0)
	}

	globalFlags := parseGlobalFlags()

	appCtx, err := app.NewContext(globalFlags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing: %v\n", err)
		os.Exit(1)
	}
	defer appCtx.Close()

	cmdArgs := filterGlobalFlags(os.Args[2:])

	if err := dispatch(cmd, appCtx, cmdArgs); err != nil {
		appCtx.Logger.Error("command failed", "cmd", cmd, "error", err)
		os.Exit(1)
	}
}

func parseGlobalFlags() app.GlobalFlags {
	var flags app.GlobalFlags

	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--log-file":
			if i+1 < len(os.Args) {
				flags.LogFile = os.Args[i+1]
				i++
			}
		case "--debug":
			flags.Debug = true
		case "--no-color":
			flags.NoColor = true
		case "--force-color":
			flags.ForceColor = true
		}
	}

	if flags.LogFile == "" {
		flags.LogFile = "./hermes.log"
	}

	return flags
}

func filterGlobalFlags(args []string) []string {
	var filtered []string
	skip := false
	for i, arg := range args {
		if skip {
			skip = false
			continue
		}
		switch arg {
		case "--log-file":
			skip = true
			continue
		case "--debug", "--no-color", "--force-color":
			continue
		default:
			if i > 0 && args[i-1] == "--log-file" {
				continue
			}
			filtered = append(filtered, arg)
		}
	}
	return filtered
}

type CommandFunc func(ctx *app.AppContext, args []string) error

var commandRegistry = map[string]CommandFunc{
	"doctor":  commands.Doctor,
	"install": commands.Install,
	"serve":   commands.Serve,
	"verify":  commands.Verify,
	"studio":  commands.Studio,
	"run":     commands.Run,
}

func dispatch(cmd string, ctx *app.AppContext, args []string) error {
	handler, ok := commandRegistry[cmd]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(2)
	}
	return handler(ctx, args)
}

func printVersion() {
	fmt.Printf("hermes %s\n", Version)
	fmt.Printf("  commit:  %s\n", Commit)
	fmt.Printf("  built:   %s\n", BuildDate)
}

func printUsage() {
	fmt.Print(ui.Banner())
	fmt.Println()
	fmt.Println("GPU inference server launcher for sglang and vllm")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  hermes <command> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  doctor    Check GPU, CUDA, and system requirements")
	fmt.Println("  install   Install inference engines (sglang, vllm)")
	fmt.Println("  serve     Start inference server")
	fmt.Println("  verify    Verify server is responding")
	fmt.Println("  studio    Launch vllm-studio controller")
	fmt.Println("  run       Run full pipeline (doctor → install → serve → verify)")
	fmt.Println("  version   Show version information")
	fmt.Println("  help      Show this help message")
	fmt.Println()
	fmt.Println("Global Flags:")

	fs := flag.NewFlagSet("global", flag.ContinueOnError)
	fs.String("log-file", "./hermes.log", "Log file path")
	fs.Bool("debug", false, "Enable debug logging")
	fs.Bool("no-color", false, "Disable colored output")
	fs.Bool("force-color", false, "Force colored output")
	fs.PrintDefaults()

	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  hermes doctor --json")
	fmt.Println("  hermes install --install sglang")
	fmt.Println("  hermes serve --engine vllm --model meta-llama/Llama-3-8B --tp 4")
	fmt.Println("  hermes run --engine sglang --model mymodel --daemon")
	fmt.Println()
	fmt.Println("For command-specific help:")
	fmt.Println("  hermes <command> --help")
}
