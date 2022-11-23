package podutils

import (
	"crypto/sha1"
	"encoding/base64"
)

const TimeFormatStr = "20060102_150405"

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

// ternary function shit
func Tern[T any](cond bool, t T, f T) T {
	if cond {
		return t
	} else {
		return f
	}
}

// split slice into chunks of set size
func Chunk[T any](slice []T, chunksize int) [][]T {
	var chunks [][]T

	for {
		if len(slice) == 0 {
			break
		}

		// check to make sure we don't split past current length
		if len(slice) < chunksize {
			chunksize = len(slice)
		}
		chunks = append(chunks, slice[0:chunksize])
		slice = slice[chunksize:]
	}

	return chunks
}
