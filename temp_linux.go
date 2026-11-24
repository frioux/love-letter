package main

import (
	"os"
	"syscall"
)

// readTempC reads the PaPiRus HAT's LM75B temperature sensor, rounded to the
// nearest degree Celsius.
func readTempC() (int, error) {
	const (
		i2cSlave     = 0x0703 // ioctl to select which device on the bus to talk to
		lm75bAddress = 0x48
		tempRegister = 0
	)

	f, err := os.OpenFile("/dev/i2c-1", os.O_RDWR, 0)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), i2cSlave, lm75bAddress); errno != 0 {
		return 0, errno
	}
	if _, err := f.Write([]byte{tempRegister}); err != nil {
		return 0, err
	}
	var raw [2]byte
	if _, err := f.Read(raw[:]); err != nil {
		return 0, err
	}

	// The register is a big-endian signed value in 1/256ths of a degree.
	return (int(int16(uint16(raw[0])<<8|uint16(raw[1]))) + 128) >> 8, nil
}
