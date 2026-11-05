//go:build !darwin

package app

func discoverWirelessServicesNative() []wirelessService { return nil }
