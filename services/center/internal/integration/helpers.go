package integration

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func buildIntegrationCode(prefix string) string {
	buffer := make([]byte, 4)
	_, _ = rand.Read(buffer)
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixNano(), hex.EncodeToString(buffer))
}
