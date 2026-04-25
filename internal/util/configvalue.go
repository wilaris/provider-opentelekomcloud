package util

import "strconv"

// ParseAnyType attempts to coerce a user-supplied string into its natural typed
// form: bool, int64, or float64. Falls back to the original string if no
// conversion succeeds.
func ParseAnyType(v string) any {
	if v == "" {
		return v
	}
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}
	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return v
}
