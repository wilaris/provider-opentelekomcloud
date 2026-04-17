package util

import (
	"maps"
	"slices"
)

// IsOptionalUpToDate reports whether an optional pointer-typed desired value
// matches the observed value. A nil desired value is always considered
// up-to-date (the field was not specified by the user).
func IsOptionalUpToDate[T comparable](desired *T, observed T) bool {
	return desired == nil || *desired == observed
}

// IsOptionalSliceUpToDate reports whether an optional slice-typed desired
// value matches the observed value. A nil desired value is always considered
// up-to-date.
func IsOptionalSliceUpToDate[S ~[]E, E comparable](desired S, observed S) bool {
	return desired == nil || slices.Equal(desired, observed)
}

// IsOptionalMapUpToDate reports whether an optional map-typed desired value
// matches the observed value. A nil desired value is always considered
// up-to-date.
func IsOptionalMapUpToDate[M ~map[K]V, K, V comparable](desired M, observed M) bool {
	return desired == nil || maps.Equal(desired, observed)
}
