package engine

import (
	"reflect"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"whitespace only", "   \t ", nil},
		{"simple flags", "--a --b", []string{"--a", "--b"}},
		{"key value", "--port 8000", []string{"--port", "8000"}},
		{"double quoted value with space", `--chat-template "a b c"`, []string{"--chat-template", "a b c"}},
		{"single quoted value with space", `--x 'y z'`, []string{"--x", "y z"}},
		{"quoted joined to flag", `--name="a b"`, []string{"--name=a b"}},
		{"empty quoted arg preserved", `--x ""`, []string{"--x", ""}},
		{"extra internal spaces", "--a    --b", []string{"--a", "--b"}},
		{"mixed quotes", `--a "b c" --d 'e f'`, []string{"--a", "b c", "--d", "e f"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseArgs(tt.in)
			if err != nil {
				t.Fatalf("ParseArgs(%q) error = %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseArgs(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseArgsRejectsUnterminatedQuotes(t *testing.T) {
	if _, err := ParseArgs(`--x "unterminated`); err == nil {
		t.Fatal("ParseArgs() error = nil, want unterminated quote error")
	}
}
