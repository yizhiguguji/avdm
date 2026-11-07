//go:build !darwin

package app

import "fmt"

func unlockSecretRead(string) (string, error) {
	return "", fmt.Errorf("解锁凭据存储当前仅支持 macOS 钥匙串")
}
func unlockSecretSave(string, string) error {
	return fmt.Errorf("解锁凭据存储当前仅支持 macOS 钥匙串")
}
func unlockSecretDelete(string) error {
	return fmt.Errorf("解锁凭据存储当前仅支持 macOS 钥匙串")
}
