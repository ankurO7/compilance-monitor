package idgen

import (
	"crypto/rand"
	"fmt"
)

func NewUUID() string {

	b := make([]byte, 16)
	_, err := rand.Read(b);

	if err != nil {
		panic(err)
	}

	// top 4 bits - 0100
	b[6] = (b[6] & 0x0f) | 0x40

	// top 2 bits - 10
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}