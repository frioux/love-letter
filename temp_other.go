//go:build !linux

package main

import "errors"

func readTempC() (int, error) {
	return 0, errors.New("no temperature sensor on this platform")
}
