package config

import "testing"

func TestDefaultServeConfig(t *testing.T) {
	c := DefaultServeConfig()
	if c.Engine != EngineSGLang {
		t.Errorf("Engine = %q, want sglang", c.Engine)
	}
	if c.TP != 1 {
		t.Errorf("TP = %d, want 1", c.TP)
	}
	if c.Host != "0.0.0.0" {
		t.Errorf("Host = %q, want 0.0.0.0", c.Host)
	}
	if c.Port != 30000 {
		t.Errorf("Port = %d, want 30000", c.Port)
	}
}

func TestValidateTP(t *testing.T) {
	tests := []struct {
		name     string
		tp       int
		gpuCount int
		wantErr  bool
	}{
		{
			name:     "zero tensor parallel size",
			tp:       0,
			gpuCount: 4,
			wantErr:  true,
		},
		{
			name:     "one tensor parallel worker with no GPUs",
			tp:       1,
			gpuCount: 0,
			wantErr:  true,
		},
		{
			name:     "tensor parallel size exceeds GPUs",
			tp:       4,
			gpuCount: 2,
			wantErr:  true,
		},
		{
			name:     "tensor parallel size matches GPUs",
			tp:       2,
			gpuCount: 2,
			wantErr:  false,
		},
		{
			name:     "unknown GPU count",
			tp:       2,
			gpuCount: -1,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTP(tt.tp, tt.gpuCount)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTP(%d, %d) error = %v, wantErr %t", tt.tp, tt.gpuCount, err, tt.wantErr)
			}
		})
	}
}

func TestDefaultVerifyConfig(t *testing.T) {
	c := DefaultVerifyConfig()
	if c.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want 127.0.0.1", c.Host)
	}
	if c.Port != 30000 {
		t.Errorf("Port = %d, want 30000", c.Port)
	}
	if c.Timeout != 60 {
		t.Errorf("Timeout = %d, want 60", c.Timeout)
	}
}

func TestDefaultInstallConfig(t *testing.T) {
	if DefaultInstallConfig().Mode != InstallBoth {
		t.Errorf("Mode = %q, want both", DefaultInstallConfig().Mode)
	}
}

func TestParseEngine(t *testing.T) {
	for _, want := range []Engine{EngineSGLang, EngineVLLM, EngineLlamaCpp} {
		got, err := ParseEngine(string(want))
		if err != nil || got != want {
			t.Errorf("ParseEngine(%q) = %q, %v; want %q, nil", want, got, err, want)
		}
	}
	if _, err := ParseEngine("nope"); err == nil {
		t.Fatal("ParseEngine(nope) error = nil")
	}
}

func TestDefaultStudioConfig(t *testing.T) {
	c := DefaultStudioConfig()
	if !c.Enabled {
		t.Error("Enabled = false, want true")
	}
	if c.Port != 8000 {
		t.Errorf("Port = %d, want 8000", c.Port)
	}
}
