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