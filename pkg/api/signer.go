package api

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mrand "math/rand"
	"time"
)

const (
	KeyAExternal  = "b83430ab79349925b33ad7cd774face8"
	KeyBInternal  = "cc52cd619a6842a679f4e8ef904824c6"
	DefaultAPIKey = "f34b59ac9857e9bbf6a7d58a5e35996b"
)

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		r := mrand.New(mrand.NewSource(time.Now().UnixNano()))
		r.Read(b)
	}
	return b
}

// GenerateCallID 动态生成 32 位随机 hex 字符串
func GenerateCallID() string {
	return hex.EncodeToString(randomBytes(16))
}

// GenerateUUID 动态生成标准 UUID v4 (带连字符，用于 devicenumber)
func GenerateUUID() string {
	b := randomBytes(16)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// GenerateSignature 计算接口签名 MD5(api_key=...call_id=...method=...app_signature=KEY_A + KEY_B)
func GenerateSignature(apiKey, callID, method string) string {
	raw := fmt.Sprintf("api_key=%scall_id=%smethod=%sapp_signature=%s%s",
		apiKey, callID, method, KeyAExternal, KeyBInternal)
	hash := md5.Sum([]byte(raw))
	return hex.EncodeToString(hash[:])
}
