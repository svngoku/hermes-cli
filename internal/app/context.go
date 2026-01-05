package app

import (
	"context"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

type AppContext struct {
	Ctx       context.Context
	Cancel    context.CancelFunc
	Logger    *log.Logger
	Stdout    io.Writer
	Stderr    io.Writer
	Debug     bool
	NoColor   bool
	LogFile   string
	logWriter io.Writer
}

type GlobalFlags struct {
	LogFile    string
	Debug      bool
	NoColor    bool
	ForceColor bool
}

func NewContext(flags GlobalFlags) (*AppContext, error) {
	ctx, cancel := context.WithCancel(context.Background())

	var logWriter io.Writer = os.Stderr
	if flags.LogFile != "" {
		f, err := os.OpenFile(flags.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			cancel()
			return nil, err
		}
		logWriter = io.MultiWriter(os.Stderr, f)
	}

	logger := log.NewWithOptions(logWriter, log.Options{
		ReportTimestamp: true,
		ReportCaller:    flags.Debug,
	})

	if flags.Debug {
		logger.SetLevel(log.DebugLevel)
	} else {
		logger.SetLevel(log.InfoLevel)
	}

	if flags.NoColor {
		logger.SetStyles(log.DefaultStyles())
		lipgloss.SetColorProfile(0)
	}
	if flags.ForceColor {
		lipgloss.SetHasDarkBackground(true)
	}

	return &AppContext{
		Ctx:       ctx,
		Cancel:    cancel,
		Logger:    logger,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Debug:     flags.Debug,
		NoColor:   flags.NoColor,
		LogFile:   flags.LogFile,
		logWriter: logWriter,
	}, nil
}

func (a *AppContext) Close() {
	a.Cancel()
	if closer, ok := a.logWriter.(io.Closer); ok && a.logWriter != os.Stderr {
		closer.Close()
	}
}
