//go:build !darwin

package app

import "context"

func findQRPairingServiceNative(context.Context, string) string { return "" }
