package ratio_setting

import "testing"

func TestGPT5CompletionRatio(t *testing.T) {
	tests := []struct {
		name string
		want float64
	}{
		{name: "gpt-5", want: 6},
		{name: "gpt-5-2025-08-07", want: 6},
		{name: "gpt-5-chat-latest", want: 6},
		{name: "gpt-5-mini", want: 6},
		{name: "gpt-5-nano", want: 6},
		{name: "gpt-5.5", want: 6},
		{name: "gpt-5.4", want: 6},
		{name: "gpt-5.4-nano", want: 6.25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCompletionRatio(tt.name); got != tt.want {
				t.Fatalf("GetCompletionRatio(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
