package config

import "testing"

func TestDefaultServeConfig(t *testing.T) {
	c := DefaultServeConfig()
	if c.Engine != EngineSGLang {
		t.Errorf("Engine = %q, want sglang", c.Engine)
	}
	if c.TP != 4 {
		t.Errorf("TP = %d, want 4", c.TP)
	}
	if c.Host != "0.0.0.0" {
		t.Errorf("Host = %q, want 0.0.0.0", c.Host)
	}
	if c.Port != 30000 {
		t.Errorf("Port = %d, want 30000", c.Port)
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

func TestDefaultStudioConfig(t *testing.T) {
	c := DefaultStudioConfig()
	if !c.Enabled {
		t.Error("Enabled = false, want true")
	}
	if c.Port != 8000 {
		t.Errorf("Port = %d, want 8000", c.Port)
	}
}
