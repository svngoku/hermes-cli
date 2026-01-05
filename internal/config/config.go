package config

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
	Engine    Engine
	Model     string
	TP        int
	Host      string
	Port      int
	Daemon    bool
	ExtraArgs string
	LogFile   string
}

type DoctorConfig struct {
	JSON   bool
	Strict bool
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
		TP:     4,
		Host:   "0.0.0.0",
		Port:   30000,
	}
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
