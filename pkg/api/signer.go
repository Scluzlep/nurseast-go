package api

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const (
	KeyAExternal  = "b83430ab79349925b33ad7cd774face8"
	KeyBInternal  = "cc52cd619a6842a679f4e8ef904824c6"
	DefaultAPIKey = "f34b59ac9857e9bbf6a7d58a5e35996b"
)

// GenerateCallID 生成 32 位的随机 hex 字符串
func GenerateCallID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "47ad647d2d7b83d6239909d712b2e021"
	}
	return hex.EncodeToString(bytes)
}

// GenerateSignature 计算接口签名 MD5(api_key=...call_id=...method=...app_signature=KEY_A + KEY_B)
func GenerateSignature(apiKey, callID, method string) string {
	raw := fmt.Sprintf("api_key=%scall_id=%smethod=%sapp_signature=%s%s",
		apiKey, callID, method, KeyAExternal, KeyBInternal)
	hash := md5.Sum([]byte(raw))
	return hex.EncodeToString(hash[:])
}
