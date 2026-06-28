package pidfile

import (
	"testing"
)

func TestWriteReadRemove(t *testing.T) {
	dir := t.TempDir()

	// Override the records directory to a temp location.
	orig := dirOverride
	dirOverride = dir
	defer func() { dirOverride = orig }()

	r := Record{PID: 12345, Engine: "vllm", Model: "m", Host: "0.0.0.0", Port: 30000}
	if err := Write(r); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(30000)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.PID != r.PID || got.Engine != r.Engine || got.Port != r.Port {
		t.Errorf("Read = %#v, want %#v", got, r)
	}

	list, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Port != 30000 {
		t.Errorf("List = %#v, want one record on 30000", list)
	}

	if err := Remove(30000); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := Read(30000); err == nil {
		t.Error("Read after Remove succeeded, want error")
	}
}

func TestRemoveMissingIsNotAnError(t *testing.T) {
	orig := dirOverride
	dirOverride = t.TempDir()
	defer func() { dirOverride = orig }()

	if err := Remove(9999); err != nil {
		t.Errorf("Remove on missing record = %v, want nil", err)
	}
}

func TestAlive(t *testing.T) {
	if Alive(0) {
		t.Error("Alive(0) = true, want false")
	}
	if !Alive(1) {
		t.Error("Alive(1) = false, want true (init always exists on unix)")
	}
}
