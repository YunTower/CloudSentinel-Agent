package crypto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

// GenerateKeyPair 生成 RSA 密钥对（2048位）
// 返回私钥和公钥的 PEM 格式字节数组
func GenerateKeyPair() (privateKey, publicKey []byte, err error) {
	// 生成 2048 位 RSA 密钥对
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("生成密钥对失败: %w", err)
	}

	// 编码私钥为 PEM 格式
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(key)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// 编码公钥为 PEM 格式
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("编码公钥失败: %w", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return privateKeyPEM, publicKeyPEM, nil
}

// GetPublicKeyFingerprint 计算公钥指纹（SHA256）
func GetPublicKeyFingerprint(publicKey []byte) (string, error) {
	// 解析 PEM 格式的公钥
	block, _ := pem.Decode(publicKey)
	if block == nil {
		return "", errors.New("无效的公钥格式")
	}

	// 解析公钥
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("解析公钥失败: %w", err)
	}

	// 将公钥编码为 DER 格式
	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("编码公钥失败: %w", err)
	}

	// 计算 SHA256 哈希
	hash := sha256.Sum256(pubDER)
	// 返回十六进制字符串（64字符）
	return fmt.Sprintf("%x", hash), nil
}

// EncryptWithPublicKey 使用公钥加密数据
func EncryptWithPublicKey(data []byte, publicKey []byte) ([]byte, error) {
	// 解析 PEM 格式的公钥
	block, _ := pem.Decode(publicKey)
	if block == nil {
		return nil, errors.New("无效的公钥格式")
	}

	// 解析公钥
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析公钥失败: %w", err)
	}

	// 类型断言为 RSA 公钥
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("不是有效的 RSA 公钥")
	}

	// RSA 加密（使用 OAEP）
	encrypted, err := rsa.EncryptOAEP(
		sha256.New(),
		rand.Reader,
		rsaPub,
		data,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("加密失败: %w", err)
	}

	return encrypted, nil
}

// DecryptWithPrivateKey 使用私钥解密数据
func DecryptWithPrivateKey(encryptedData []byte, privateKey []byte) ([]byte, error) {
	// 解析 PEM 格式的私钥
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return nil, errors.New("无效的私钥格式")
	}

	// 解析私钥
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	// RSA 解密（使用 OAEP）
	decrypted, err := rsa.DecryptOAEP(
		sha256.New(),
		rand.Reader,
		key,
		encryptedData,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}

	return decrypted, nil
}

// SignData 使用私钥对数据进行签名
func SignData(data []byte, privateKey []byte) ([]byte, error) {
	// 解析 PEM 格式的私钥
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return nil, errors.New("无效的私钥格式")
	}

	// 解析私钥
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	// 计算数据的哈希值
	hash := sha256.Sum256(data)

	// 使用私钥签名
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return nil, fmt.Errorf("签名失败: %w", err)
	}

	return signature, nil
}

// VerifySignature 验证签名
func VerifySignature(data, signature []byte, publicKey []byte) (bool, error) {
	// 解析 PEM 格式的公钥
	block, _ := pem.Decode(publicKey)
	if block == nil {
		return false, errors.New("无效的公钥格式")
	}

	// 解析公钥
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return false, fmt.Errorf("解析公钥失败: %w", err)
	}

	// 类型断言为 RSA 公钥
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return false, errors.New("不是有效的 RSA 公钥")
	}

	// 计算数据的哈希值
	hash := sha256.Sum256(data)

	// 验证签名
	err = rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, hash[:], signature)
	if err != nil {
		return false, nil // 签名验证失败，但不返回错误
	}

	return true, nil
}
