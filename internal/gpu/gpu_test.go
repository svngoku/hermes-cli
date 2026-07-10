package gpu

import "testing"

func TestCountFromQueryOutput(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
		want   int
	}{
		{
			name:   "empty output",
			stdout: "",
			want:   0,
		},
		{
			name:   "one GPU",
			stdout: "0, NVIDIA H100, 81559 MiB",
			want:   1,
		},
		{
			name: "two GPUs",
			stdout: "0, NVIDIA H100, 81559 MiB\n" +
				"1, NVIDIA H100, 81559 MiB",
			want: 2,
		},
		{
			name: "blank and trailing lines",
			stdout: "\n0, NVIDIA H100, 81559 MiB\n\n" +
				"1, NVIDIA H100, 81559 MiB\n\n",
			want: 2,
		},
		{
			name:   "whitespace lines",
			stdout: " \n\t\n  \r\n",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CountFromQueryOutput(tt.stdout); got != tt.want {
				t.Errorf("CountFromQueryOutput() = %d, want %d", got, tt.want)
			}
		})
	}
}
