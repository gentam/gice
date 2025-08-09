package main

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/host/v3"
	"periph.io/x/host/v3/ftdi"
)

type Device struct {
	ft *ftdi.FT232H
	cs gpio.PinIO

	clock physic.Frequency
	conn  spi.Conn
}

var hostInitialized atomic.Bool

// NewDevice opens FT2232H and initializes SPI connection.
func NewDevice() (*Device, error) {
	if hostInitialized.CompareAndSwap(false, true) {
		if _, err := host.Init(); err != nil {
			return nil, fmt.Errorf("host initialization failed: %w", err)
		}
	}

	d := &Device{
		// [AS_135 3.2.1 Divisors] specifies range in [92Hz, 30MHz], but
		// periph.io's minimum is 100Hz
		clock: 30 * physic.MegaHertz,
	}
	if err := d.openFT2232H(); err != nil {
		return nil, err
	}

	// [EB82|Appendix A. Sheet 2 of 5 (USB to SPI/RS232)]
	// ADBUS0 | 16 | iCE_SCK
	// ADBUS1 | 17 | iCE_SI
	// ADBUS2 | 18 | iCE_SO
	// ADBUS4 | 21 | iCE_SS_B (CS)
	// ADBUS6 | 23 | iCE_CDONE
	// ADBUS7 | 24 | iCE_RESET
	d.cs = d.ft.D4 // ADBUS4 (GPIOLO → CS)

	if err := d.connectSPI(); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *Device) openFT2232H() error {
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
			d.ft = ft
			return nil
		}
	}

	return errors.New("not found")
}

func (d *Device) connectSPI() (err error) {
	if d.ft == nil {
		return errors.New("FT2232H device not found")
	}

	port, err := d.ft.SPI()
	if err != nil {
		return fmt.Errorf("failed to get SPI port: %w", err)
	}

	// Mode0 and Mode3 are supported [n25q_32mb_3v_65nm.pdf|Table 7: SPI Modes]
	mode := spi.Mode0
	d.conn, err = port.Connect(d.clock, mode, 8)
	return err
}

func (d *Device) releasePowerDown() error {
	buf := []byte{flashCmdReleasePowerDown, 0, 0, 0, 0}
	if err := d.cs.Out(gpio.Low); err != nil {
		return err
	}
	if err := d.conn.Tx(buf, buf); err != nil {
		d.cs.Out(gpio.High)
		return err
	}
	if err := d.cs.Out(gpio.High); err != nil {
		return err
	}
	time.Sleep(3 * time.Microsecond) // [W25Q128JV-DTR|9.6 AC Electrical Characteristics: tRES1]
	return nil
}
