package util

import "encoding/base64"

// Base64IfNot returns s unchanged if it is already standard base64-encoded,
// otherwise it returns s encoded as standard base64.
func Base64IfNot(s string) string {
	if _, err := base64.StdEncoding.DecodeString(s); err == nil {
		return s
	}
	return base64.StdEncoding.EncodeToString([]byte(s))
}
