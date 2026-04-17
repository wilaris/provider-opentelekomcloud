package util

import (
	"maps"
	"slices"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// LateInitPtr returns current if non-nil; otherwise it returns a pointer
// to observed and marks the initializer as changed.
// Use this for fields where the zero value is meaningful (e.g. bools).
func LateInitPtr[T comparable](current *T, observed T, li *resource.LateInitializer) *T {
	if current != nil {
		return current
	}
	v := observed
	li.SetChanged()
	return &v
}

// LateInitPtrIfNonZero returns current if non-nil; if current is nil and
// observed is not the zero value, it returns a pointer to observed and marks
// the initializer as changed. Returns nil when observed is the zero value.
// Use this for fields where the zero value means "unset" (e.g. strings).
func LateInitPtrIfNonZero[T comparable](current *T, observed T, li *resource.LateInitializer) *T {
	var zero T
	if current != nil || observed == zero {
		return current
	}
	v := observed
	li.SetChanged()
	return &v
}

// LateInitSliceIfNonEmpty returns current if non-nil; otherwise if observed
// has at least one element, it returns a clone of observed and marks the
// initializer as changed.
func LateInitSliceIfNonEmpty[S ~[]E, E comparable](
	current S,
	observed S,
	li *resource.LateInitializer,
) S {
	if current != nil || len(observed) == 0 {
		return current
	}
	li.SetChanged()
	return slices.Clone(observed)
}

// LateInitMapIfNonEmpty returns current if non-nil; otherwise if observed
// has at least one entry, it returns a clone of observed and marks the
// initializer as changed.
func LateInitMapIfNonEmpty[M ~map[K]V, K, V comparable](
	current M,
	observed M,
	li *resource.LateInitializer,
) M {
	if current != nil || len(observed) == 0 {
		return current
	}
	li.SetChanged()
	return maps.Clone(observed)
}
