package domain

import "testing"

func TestHledgerVersionAtLeast(t *testing.T) {
	floor := HledgerVersion{Major: 1, Minor: 32}

	tests := []struct {
		name string
		v    HledgerVersion
		want bool
	}{
		{"equal to the floor", HledgerVersion{Major: 1, Minor: 32}, true},
		{"same major, minor one below", HledgerVersion{Major: 1, Minor: 31}, false},
		{"same major, minor above", HledgerVersion{Major: 1, Minor: 40}, true},
		{"major above", HledgerVersion{Major: 2, Minor: 0}, true},
		{"major below", HledgerVersion{Major: 0, Minor: 99}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.AtLeast(floor); got != tt.want {
				t.Errorf("%+v.AtLeast(%+v) = %v, want %v", tt.v, floor, got, tt.want)
			}
		})
	}
}
