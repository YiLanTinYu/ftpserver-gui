package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const passwordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%"

func generatePassword(length int) (string, error) {
	if length < 1 {
		return "", fmt.Errorf("password length must be positive")
	}
	result := make([]byte, length)
	limit := big.NewInt(int64(len(passwordAlphabet)))
	for i := range result {
		index, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		result[i] = passwordAlphabet[index.Int64()]
	}
	return string(result), nil
}
