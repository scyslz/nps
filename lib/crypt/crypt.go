package crypt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"math/rand"
	"time"

	"golang.org/x/crypto/blake2b"
)

// en
func AesEncrypt(origData, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	origData = PKCS5Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, key[:blockSize])
	crypted := make([]byte, len(origData))
	blockMode.CryptBlocks(crypted, origData)
	return crypted, nil
}

// de
func AesDecrypt(crypted, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, key[:blockSize])
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, crypted)
	err, origData = PKCS5UnPadding(origData)
	return origData, err
}

// Completion when the length is insufficient
func PKCS5Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

// Remove excess
func PKCS5UnPadding(origData []byte) (error, []byte) {
	length := len(origData)
	unpadding := int(origData[length-1])
	if (length - unpadding) < 0 {
		return errors.New("len error"), nil
	}
	return nil, origData[:(length - unpadding)]
}

// Generate 32-bit MD5 strings
func Md5(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// Generate 32-bit BLAKE2b-256 strings
func Blake2b(s string) string {
	hash := blake2b.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// GetRandomString 生成指定长度的随机密钥，支持可选传入id
func GetRandomString(l int, id ...int) string {
	// 字符集
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	var result []byte

	// 如果传入id，则将id转换为字符集映射并倒序放在最前面
	if len(id) > 0 {
		// 将id转为字符集表示的字符串
		idMapped := ""
		for id[0] > 0 {
			idMapped = string(str[id[0]%len(str)]) + idMapped
			id[0] /= len(str)
		}

		// 如果倒序后的id长度超过指定长度l，则截断
		//if len(idMapped) > l {
		//	idMapped = idMapped[:l]
		//}

		// 将倒序后的id添加到结果中
		result = append(result, []byte(idMapped)...)
	}

	// 计算剩余需要生成的随机字符的长度
	remainingLength := l - len(result)
	if remainingLength > 0 {
		// 使用当前时间的UnixNano作为随机数种子
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		// 生成剩余的随机字符
		for i := 0; i < remainingLength; i++ {
			result = append(result, bytes[r.Intn(len(bytes))])
		}
	}

	// 返回最终结果字符串
	return string(result)
}
