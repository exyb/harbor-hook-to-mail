package tests

import (
	"encoding/base64"
	"fmt"
	"log"
	"testing"

	. "github.com/exyb/harbor-hook-to-mail/utils"
)

func TestEncrypt(t *testing.T) {
	plaintextBytes := []byte("hook@123A")

	ciphertext, err := EncryptAES(plaintextBytes)
	if err != nil {
		fmt.Println(err)
		return
	}
	encryptedBase64 := base64.StdEncoding.EncodeToString(ciphertext)
	fmt.Printf("Ciphertext (Base64 encoded): %s\n", encryptedBase64)

	encryptedPassword, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		log.Fatalf("Failed to decode registry password from base64: %v", err)
	}

	decryptedPassword, err := DecryptAES(encryptedPassword)
	if err != nil {
		log.Fatalf("Failed to decode registry password: %v", err)
	}

	fmt.Printf("Ciphertext (Base64 encoded): %s\n", decryptedPassword)

}
