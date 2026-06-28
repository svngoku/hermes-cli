// Package pidfile persists records of background (daemon) engine processes so
// they can be listed and stopped after the launching CLI has exited. Records
// are keyed by port and stored as JSON under ~/.cache/hermes/daemons.
package pidfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Record describes one launched engine process.
type Record struct {
	PID       int       `json:"pid"`
	Engine    string    `json:"engine"`
	Model     string    `json:"model"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	LogFile   string    `json:"log_file,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

// dirOverride lets tests redirect the records directory. When empty, the
// default ~/.cache/hermes/daemons is used.
var dirOverride string

func dir() string {
	if dirOverride != "" {
		return dirOverride
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "hermes", "daemons")
}

func path(port int) string {
	return filepath.Join(dir(), fmt.Sprintf("%d.json", port))
}

// Write persists a record, overwriting any existing entry for the same port.
func Write(r Record) error {
	if err := os.MkdirAll(dir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path(r.Port), data, 0644)
}

// Read returns the record for a port.
func Read(port int) (Record, error) {
	var r Record
	data, err := os.ReadFile(path(port))
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return r, fmt.Errorf("corrupt pid record for port %d: %w", port, err)
	}
	return r, nil
}

// Remove deletes the record for a port. A missing record is not an error.
func Remove(port int) error {
	err := os.Remove(path(port))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// List returns all known records sorted by port.
func List() ([]Record, error) {
	entries, err := os.ReadDir(dir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var records []Record
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir(), e.Name()))
		if err != nil {
			continue
		}
		var r Record
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		records = append(records, r)
	}

	sort.Slice(records, func(i, j int) bool { return records[i].Port < records[j].Port })
	return records, nil
}

// Alive reports whether a process with the given pid currently exists.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Signal 0 performs error checking without delivering a signal.
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
