package config

import "fmt"

type Engine string

const (
	EngineSGLang Engine = "sglang"
	EngineVLLM   Engine = "vllm"
)

type InstallMode string

const (
	InstallSGLang InstallMode = "sglang"
	InstallVLLM   InstallMode = "vllm"
	InstallBoth   InstallMode = "both"
	InstallNone   InstallMode = "none"
)

type ServeConfig struct {
	Engine      Engine
	Model       string
	TP          int
	Host        string
	Port        int
	Daemon      bool
	CUDADevices string
	ExtraArgs   string
	LogFile     string
}

type InstallConfig struct {
	Mode  InstallMode
	Check bool
}

type VerifyConfig struct {
	Host    string
	Port    int
	Timeout int
	Skip    bool
}

type StudioConfig struct {
	Enabled  bool
	Port     int
	Frontend bool
}

func DefaultServeConfig() ServeConfig {
	return ServeConfig{
		Engine: EngineSGLang,
		TP:     1,
		Host:   "0.0.0.0",
		Port:   30000,
	}
}

func ValidateTP(tp, gpuCount int) error {
	if tp < 1 {
		return fmt.Errorf("tensor parallel size must be at least 1 (got %d; available GPU count: %d)", tp, gpuCount)
	}
	if gpuCount >= 0 && tp > gpuCount {
		return fmt.Errorf("tensor parallel size %d exceeds available GPU count %d", tp, gpuCount)
	}
	return nil
}

func DefaultInstallConfig() InstallConfig {
	return InstallConfig{
		Mode: InstallBoth,
	}
}

func DefaultVerifyConfig() VerifyConfig {
	return VerifyConfig{
		Host:    "127.0.0.1",
		Port:    30000,
		Timeout: 60,
	}
}

func DefaultStudioConfig() StudioConfig {
	return StudioConfig{
		Enabled: true,
		Port:    8000,
	}
}
