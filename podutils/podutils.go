package podutils

import (
	"crypto/sha1"
	"encoding/base64"
)

func GenerateHash(str string) string {
	sha := sha1.New()
	sha.Write([]byte(str))
	hash := base64.URLEncoding.EncodeToString(sha.Sum(nil))

	return hash
}

// appends to a slice (array) without modifying the underlying array;
// creates copy of the underlying array and adds the new entry to it
func CopyAndAppend[T any](src []T, add ...T) []T {
	// copy the underlying array directly
	var dst = make([]T, len(src))
	copy(dst, src)
	dst = append(dst, add...)
	return dst
}
