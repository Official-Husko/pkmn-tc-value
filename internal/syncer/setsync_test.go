package syncer

import "testing"

func TestConfiguredImageWorkers(t *testing.T) {
	tests := []struct {
		name       string
		configured int
		total      int
		want       int
	}{
		{name: "no cards", configured: 8, total: 0, want: 0},
		{name: "min clamp", configured: 0, total: 10, want: 1},
		{name: "as configured", configured: 4, total: 10, want: 4},
		{name: "max clamp to total", configured: 32, total: 6, want: 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := configuredImageWorkers(tt.configured, tt.total); got != tt.want {
				t.Fatalf("configuredImageWorkers(%d, %d) = %d, want %d", tt.configured, tt.total, got, tt.want)
			}
		})
	}
}
