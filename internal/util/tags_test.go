package util

import (
	"testing"

	"github.com/opentelekomcloud/gophertelekomcloud/openstack/common/tags"
)

func TestTagDiff(t *testing.T) {
	tests := []struct {
		name string
		a    map[string]string
		b    map[string]string
		want map[string]string
	}{
		{
			name: "nil a returns nil",
			a:    nil,
			b:    map[string]string{"k": "v"},
			want: nil,
		},
		{
			name: "empty a returns nil",
			a:    map[string]string{},
			b:    map[string]string{"k": "v"},
			want: nil,
		},
		{
			name: "identical maps returns nil",
			a:    map[string]string{"env": "dev"},
			b:    map[string]string{"env": "dev"},
			want: nil,
		},
		{
			name: "new key in a",
			a:    map[string]string{"env": "dev", "team": "infra"},
			b:    map[string]string{"env": "dev"},
			want: map[string]string{"team": "infra"},
		},
		{
			name: "changed value in a",
			a:    map[string]string{"env": "prod"},
			b:    map[string]string{"env": "dev"},
			want: map[string]string{"env": "prod"},
		},
		{
			name: "extra key in b is ignored",
			a:    map[string]string{"env": "dev"},
			b:    map[string]string{"env": "dev", "team": "infra"},
			want: nil,
		},
		{
			name: "nil b returns all of a",
			a:    map[string]string{"env": "dev"},
			b:    nil,
			want: map[string]string{"env": "dev"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TagDiff(tt.a, tt.b)
			if tt.want == nil {
				if got != nil {
					t.Errorf("TagDiff() = %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("TagDiff() = %v, want %v", got, tt.want)
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("TagDiff()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestResourceTagsToMap(t *testing.T) {
	tests := []struct {
		name string
		in   []tags.ResourceTag
		want map[string]string
	}{
		{
			name: "nil input returns empty map",
			in:   nil,
			want: map[string]string{},
		},
		{
			name: "empty input returns empty map",
			in:   []tags.ResourceTag{},
			want: map[string]string{},
		},
		{
			name: "single tag",
			in:   []tags.ResourceTag{{Key: "env", Value: "dev"}},
			want: map[string]string{"env": "dev"},
		},
		{
			name: "multiple tags",
			in: []tags.ResourceTag{
				{Key: "env", Value: "dev"},
				{Key: "team", Value: "infra"},
			},
			want: map[string]string{"env": "dev", "team": "infra"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResourceTagsToMap(tt.in)
			if len(got) != len(tt.want) {
				t.Errorf("ResourceTagsToMap() = %v, want %v", got, tt.want)
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("ResourceTagsToMap()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestMapToResourceTags(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]string
		want []tags.ResourceTag
	}{
		{
			name: "nil input returns nil",
			in:   nil,
			want: nil,
		},
		{
			name: "empty input returns nil",
			in:   map[string]string{},
			want: nil,
		},
		{
			name: "single entry",
			in:   map[string]string{"env": "dev"},
			want: []tags.ResourceTag{{Key: "env", Value: "dev"}},
		},
		{
			name: "multiple entries sorted by key",
			in:   map[string]string{"zebra": "z", "alpha": "a", "mid": "m"},
			want: []tags.ResourceTag{
				{Key: "alpha", Value: "a"},
				{Key: "mid", Value: "m"},
				{Key: "zebra", Value: "z"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapToResourceTags(tt.in)
			if tt.want == nil {
				if got != nil {
					t.Errorf("MapToResourceTags() = %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("MapToResourceTags() length = %d, want %d", len(got), len(tt.want))
				return
			}
			for i, w := range tt.want {
				if got[i].Key != w.Key || got[i].Value != w.Value {
					t.Errorf("MapToResourceTags()[%d] = {%q, %q}, want {%q, %q}",
						i, got[i].Key, got[i].Value, w.Key, w.Value)
				}
			}
		})
	}
}
