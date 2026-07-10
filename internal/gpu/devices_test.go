package gpu

import "testing"

func TestParseCUDADevices(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantCount      int
		wantNormalized string
		wantErr        bool
	}{
		{
			name: "empty",
		},
		{
			name:  "whitespace",
			input: " \t\n ",
		},
		{
			name:           "one",
			input:          "0",
			wantCount:      1,
			wantNormalized: "0",
		},
		{
			name:           "two",
			input:          "0,1",
			wantCount:      2,
			wantNormalized: "0,1",
		},
		{
			name:           "spaced three",
			input:          "0, 1, 2",
			wantCount:      3,
			wantNormalized: "0,1,2",
		},
		{
			name:    "duplicate",
			input:   "0,1,0",
			wantErr: true,
		},
		{
			name:    "alphabetic",
			input:   "0,a",
			wantErr: true,
		},
		{
			name:    "empty token",
			input:   "0,,1",
			wantErr: true,
		},
		{
			name:    "negative",
			input:   "0,-1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, normalized, err := ParseCUDADevices(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseCUDADevices(%q) error = %v, wantErr %t", tt.input, err, tt.wantErr)
			}
			if count != tt.wantCount {
				t.Errorf("ParseCUDADevices(%q) count = %d, want %d", tt.input, count, tt.wantCount)
			}
			if normalized != tt.wantNormalized {
				t.Errorf("ParseCUDADevices(%q) normalized = %q, want %q", tt.input, normalized, tt.wantNormalized)
			}
		})
	}
}
