package util

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
)

func TestLateInitPtr(t *testing.T) {
	tests := []struct {
		name        string
		current     *bool
		observed    bool
		want        *bool
		wantChanged bool
	}{
		{"nil current, true observed", nil, true, pointer.To(true), true},
		{"nil current, false observed (zero)", nil, false, pointer.To(false), true},
		{"non-nil current, different observed", pointer.To(true), false, pointer.To(true), false},
		{"non-nil current, same observed", pointer.To(false), false, pointer.To(false), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			li := resource.NewLateInitializer()
			got := LateInitPtr(tt.current, tt.observed, li)

			if got == nil {
				t.Fatal("got nil, want non-nil")
			}
			if *got != *tt.want {
				t.Errorf("*got = %v, want %v", *got, *tt.want)
			}
			if li.IsChanged() != tt.wantChanged {
				t.Errorf("IsChanged() = %v, want %v", li.IsChanged(), tt.wantChanged)
			}
		})
	}
}

func TestLateInitPtrIfNonZero(t *testing.T) {
	tests := []struct {
		name        string
		current     *string
		observed    string
		want        *string
		wantChanged bool
	}{
		{"nil current, non-zero observed", nil, "hello", pointer.To("hello"), true},
		{"nil current, zero observed", nil, "", nil, false},
		{"non-nil current, different observed", pointer.To("a"), "b", pointer.To("a"), false},
		{"non-nil current, same observed", pointer.To("a"), "a", pointer.To("a"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			li := resource.NewLateInitializer()
			got := LateInitPtrIfNonZero(tt.current, tt.observed, li)

			if tt.want == nil {
				if got != nil {
					t.Fatalf("got = %v, want nil", *got)
				}
			} else {
				if got == nil {
					t.Fatal("got nil, want non-nil")
				}
				if *got != *tt.want {
					t.Errorf("*got = %v, want %v", *got, *tt.want)
				}
			}
			if li.IsChanged() != tt.wantChanged {
				t.Errorf("IsChanged() = %v, want %v", li.IsChanged(), tt.wantChanged)
			}
		})
	}
}

func TestLateInitSliceIfNonEmpty(t *testing.T) {
	tests := []struct {
		name        string
		current     []string
		observed    []string
		wantLen     int
		wantChanged bool
	}{
		{"nil current, non-empty observed", nil, []string{"a", "b"}, 2, true},
		{"nil current, empty observed", nil, []string{}, 0, false},
		{"non-nil current, non-empty observed", []string{"x"}, []string{"a", "b"}, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			li := resource.NewLateInitializer()
			got := LateInitSliceIfNonEmpty(tt.current, tt.observed, li)

			if len(got) != tt.wantLen {
				t.Errorf("len(got) = %d, want %d", len(got), tt.wantLen)
			}
			if li.IsChanged() != tt.wantChanged {
				t.Errorf("IsChanged() = %v, want %v", li.IsChanged(), tt.wantChanged)
			}
		})
	}
}

func TestLateInitMapIfNonEmpty(t *testing.T) {
	tests := []struct {
		name        string
		current     map[string]string
		observed    map[string]string
		wantLen     int
		wantChanged bool
	}{
		{"nil current, non-empty observed", nil, map[string]string{"k": "v"}, 1, true},
		{"nil current, empty observed", nil, map[string]string{}, 0, false},
		{
			"non-nil current, non-empty observed",
			map[string]string{"x": "y"},
			map[string]string{"k": "v"},
			1,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			li := resource.NewLateInitializer()
			got := LateInitMapIfNonEmpty(tt.current, tt.observed, li)

			if len(got) != tt.wantLen {
				t.Errorf("len(got) = %d, want %d", len(got), tt.wantLen)
			}
			if li.IsChanged() != tt.wantChanged {
				t.Errorf("IsChanged() = %v, want %v", li.IsChanged(), tt.wantChanged)
			}
		})
	}
}
