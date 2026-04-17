package util

import (
	"testing"
)

func TestIsOptionalUpToDate(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		s := func(v string) *string { return &v }
		tests := []struct {
			name     string
			desired  *string
			observed string
			want     bool
		}{
			{"nil desired", nil, "any", true},
			{"matching", s("hello"), "hello", true},
			{"mismatching", s("hello"), "world", false},
			{"empty desired matches empty observed", s(""), "", true},
			{"empty desired vs non-empty observed", s(""), "x", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := IsOptionalUpToDate(tt.desired, tt.observed); got != tt.want {
					t.Errorf("IsOptionalUpToDate() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("bool", func(t *testing.T) {
		b := func(v bool) *bool { return &v }
		tests := []struct {
			name     string
			desired  *bool
			observed bool
			want     bool
		}{
			{"nil desired", nil, true, true},
			{"matching true", b(true), true, true},
			{"matching false", b(false), false, true},
			{"mismatching", b(true), false, false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := IsOptionalUpToDate(tt.desired, tt.observed); got != tt.want {
					t.Errorf("IsOptionalUpToDate() = %v, want %v", got, tt.want)
				}
			})
		}
	})
}

func TestIsOptionalSliceUpToDate(t *testing.T) {
	tests := []struct {
		name     string
		desired  []string
		observed []string
		want     bool
	}{
		{"nil desired", nil, []string{"a"}, true},
		{"matching", []string{"a", "b"}, []string{"a", "b"}, true},
		{"mismatching", []string{"a"}, []string{"b"}, false},
		{"empty desired vs empty observed", []string{}, []string{}, true},
		{"empty desired vs non-empty observed", []string{}, []string{"a"}, false},
		{"different order", []string{"a", "b"}, []string{"b", "a"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOptionalSliceUpToDate(tt.desired, tt.observed); got != tt.want {
				t.Errorf("IsOptionalSliceUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsOptionalMapUpToDate(t *testing.T) {
	tests := []struct {
		name     string
		desired  map[string]string
		observed map[string]string
		want     bool
	}{
		{"nil desired", nil, map[string]string{"k": "v"}, true},
		{"matching", map[string]string{"k": "v"}, map[string]string{"k": "v"}, true},
		{"mismatching value", map[string]string{"k": "v"}, map[string]string{"k": "x"}, false},
		{
			"extra key in observed",
			map[string]string{"k": "v"},
			map[string]string{"k": "v", "k2": "v2"},
			false,
		},
		{
			"extra key in desired",
			map[string]string{"k": "v", "k2": "v2"},
			map[string]string{"k": "v"},
			false,
		},
		{"empty desired vs empty observed", map[string]string{}, map[string]string{}, true},
		{
			"empty desired vs non-empty observed",
			map[string]string{},
			map[string]string{"k": "v"},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOptionalMapUpToDate(tt.desired, tt.observed); got != tt.want {
				t.Errorf("IsOptionalMapUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}
