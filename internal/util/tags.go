package util

import (
	"slices"

	"github.com/opentelekomcloud/gophertelekomcloud/openstack/common/tags"
)

// TagDiff returns entries in a that are missing or different in b.
// Returns nil if there are no differences.
func TagDiff(a map[string]string, b map[string]string) map[string]string {
	if len(a) == 0 {
		return nil
	}

	out := map[string]string{}
	for key, value := range a {
		currentValue, ok := b[key]
		if !ok || currentValue != value {
			out[key] = value
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

// ResourceTagsToMap converts a slice of ResourceTag to a string map.
// Returns an empty map (not nil) when the input is empty.
func ResourceTagsToMap(in []tags.ResourceTag) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}

	out := make(map[string]string, len(in))
	for _, t := range in {
		out[t.Key] = t.Value
	}
	return out
}

// MapToResourceTags converts a string map to a sorted slice of ResourceTag.
// Returns nil when the input is empty.
func MapToResourceTags(in map[string]string) []tags.ResourceTag {
	if len(in) == 0 {
		return nil
	}

	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	out := make([]tags.ResourceTag, 0, len(in))
	for _, key := range keys {
		out = append(out, tags.ResourceTag{Key: key, Value: in[key]})
	}
	return out
}
