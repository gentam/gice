package gice

import (
	"errors"
	"fmt"
	"sync/atomic"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/host/v3"
	"periph.io/x/host/v3/ftdi"
)

type Device struct {
	FTDI  *ftdi.FT232H
	Flash *Flash

	cs    gpio.PinIO // ADBUS4 Chip Select
	reset gpio.PinIO // ADBUS7 Reset
	cdone gpio.PinIO // ADBUS6 Done

	clock physic.Frequency
	conn  spi.Conn
}

var hostInitialized atomic.Bool

// NewDevice finds FT2232H device and opens MPSSE/SPI connection.
func NewDevice() (*Device, error) {
	if hostInitialized.CompareAndSwap(false, true) {
		if _, err := host.Init(); err != nil {
			return nil, fmt.Errorf("host initialization failed: %w", err)
		}
	}

	d := &Device{
		clock: 30 * physic.MegaHertz, // [AN_135 3.2.1 Divisors]
	}
	if err := d.findFT2232H(); err != nil {
		return nil, err
	}

	// [EB82|Appendix A. Sheet 2 of 5 (USB to SPI/RS232)] / [icebreaker-sch.pdf]
	// ADBUS0 | iCE_SCK
	// ADBUS1 | iCE_MOSI / FLASH_MOSI
	// ADBUS2 | iCE_MISO / FLASH_MISO
	// ADBUS4 | iCE_SS_B
	// ADBUS6 | iCE_CDONE
	// ADBUS7 | iCE_CRESET / iCE_RESET
	d.cs = d.FTDI.D4
	d.reset = d.FTDI.D7
	d.cdone = d.FTDI.D6

	if err := d.connectSPI(); err != nil {
		return nil, err
	}

	d.Flash = NewFlash(d.conn, d.cs)

	return d, nil
}

// ResetFPGA asserts (low) or deasserts (high) the FPGA reset line.
func (d *Device) ResetFPGA(l gpio.Level) error {
	return d.reset.Out(l)
}

func (d *Device) findFT2232H() error {
	const (
		vendorID  = 0x0403 // FTDI
		productID = 0x6010 // FT2232H
	)

	info := ftdi.Info{}
	for _, dev := range ftdi.All() {
		dev.Info(&info)
		if info.VenID != vendorID || info.DevID != productID {
			continue
		}
		if ft, ok := dev.(*ftdi.FT232H); ok {
			d.FTDI = ft
			return nil
		}
	}

	return errors.New("not found")
}

func (d *Device) connectSPI() (err error) {
	if d.FTDI == nil {
		return errors.New("FT2232H device not found")
	}

	port, err := d.FTDI.SPI()
	if err != nil {
		return fmt.Errorf("failed to get SPI port: %w", err)
	}

	// [FTDI AN_114|1.2]> FTDI device can only support mode 0 and mode 2 due to the limitation of MPSSE engine
	// [n25q_32mb_3v_65nm.pdf|Table 7: SPI Modes] mode 0 and mode 3 are supported
	mode := spi.Mode0
	d.conn, err = port.Connect(d.clock, mode, 8)
	return err
}
