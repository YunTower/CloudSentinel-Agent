package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// EncryptMessage 使用 AES-GCM 加密消息
// 返回格式：nonce(12字节) + ciphertext + tag(16字节)
func EncryptMessage(message []byte, key []byte) ([]byte, error) {
	// 验证密钥长度（AES-256 需要 32 字节）
	if len(key) != 32 {
		return nil, errors.New("密钥长度必须是 32 字节（AES-256）")
	}

	// 创建 AES 密码块
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES 密码块失败: %w", err)
	}

	// 创建 GCM 模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 模式失败: %w", err)
	}

	// 生成随机 nonce（12 字节，GCM 推荐长度）
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成 nonce 失败: %w", err)
	}

	// 加密消息（Seal 会自动附加认证标签）
	ciphertext := gcm.Seal(nonce, nonce, message, nil)

	return ciphertext, nil
}

// DecryptMessage 使用 AES-GCM 解密消息
// 输入格式：nonce(12字节) + ciphertext + tag(16字节)
func DecryptMessage(encryptedMessage []byte, key []byte) ([]byte, error) {
	// 验证密钥长度（AES-256 需要 32 字节）
	if len(key) != 32 {
		return nil, errors.New("密钥长度必须是 32 字节（AES-256）")
	}

	// 创建 AES 密码块
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES 密码块失败: %w", err)
	}

	// 创建 GCM 模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 模式失败: %w", err)
	}

	// 检查消息长度（至少需要 nonce + tag）
	nonceSize := gcm.NonceSize()
	if len(encryptedMessage) < nonceSize {
		return nil, errors.New("加密消息长度不足")
	}

	// 提取 nonce 和密文
	nonce, ciphertext := encryptedMessage[:nonceSize], encryptedMessage[nonceSize:]

	// 解密消息（Open 会自动验证认证标签）
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}

	return plaintext, nil
}
