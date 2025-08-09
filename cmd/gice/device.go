package main

import (
	"errors"
	"fmt"
	"io"
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

func (d *Device) ReleasePowerDown() error {
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

// Flash operations

// [n25q_32mb_3v_65nm.pdf|Table 16: Command Set]
// [W25Q128JV-DTR|8.1.2 Instruction Set Table 1]
const (
	flashCmdReleasePowerDown = 0xAB
	flashCmdReadID           = 0x9F
	flashCmdRead             = 0x03
	flashCmdWriteEnable      = 0x06
	flashCmdPageProgram      = 0x02
)

var knownFlashIDs = map[[3]byte]string{
	{0x20, 0xBA, 0x16}: "Micron N25Q032",
	{0xEF, 0x70, 0x18}: "Winbond W25Q128JVIM",
}

func (d *Device) IsKnownFlashID(id [3]byte) (string, bool) {
	if name, ok := knownFlashIDs[id]; ok {
		return name, true
	}
	return "", false
}

func (d *Device) ReadFlashID() (id [3]byte, err error) {
	buf := make([]byte, 4)
	buf[0] = flashCmdReadID
	fmt.Printf("%X\n", buf)
	d.cs.Out(gpio.High)
	time.Sleep(time.Millisecond)
	if err = d.cs.Out(gpio.Low); err != nil {
		return
	}
	if err = d.conn.Tx(buf, buf); err != nil {
		d.cs.Out(gpio.High)
		return
	}
	err = d.cs.Out(gpio.High)
	time.Sleep(time.Millisecond)
	return [3]byte(buf[1:]), err
}

// ReadFlash splits the read operation into multiple transactions to avoid
// exceeding the maximum transaction size.
func (d *Device) ReadFlash(addr, n int) ([]byte, error) {
	const (
		maxTx    = 65536 // [AN_108]
		cmdBytes = 4     // opRead + 24‑bit address
		maxData  = maxTx - cmdBytes
	)

	out := make([]byte, n)
	off := 0
	for remaining := n; remaining > 0; {
		chunk := min(remaining, maxData)
		buf := make([]byte, cmdBytes+chunk)
		buf[0] = flashCmdRead
		buf[1] = byte(addr >> 16)
		buf[2] = byte(addr >> 8)
		buf[3] = byte(addr)
		// tx[4:] dummy bytes

		if err := d.cs.Out(gpio.Low); err != nil {
			return nil, err
		}
		txErr := d.conn.Tx(buf, buf)
		if csErr := d.cs.Out(gpio.High); csErr != nil {
			return nil, csErr
		}
		if txErr != nil {
			return nil, txErr
		}

		copy(out[off:], buf[cmdBytes:])

		addr += chunk
		off += chunk
		remaining -= chunk
	}
	return out, nil
}

func (d *Device) writeEnable() error {
	buf := []byte{flashCmdWriteEnable}
	if err := d.cs.Out(gpio.Low); err != nil {
		return err
	}
	if err := d.conn.Tx(buf, buf); err != nil {
		d.cs.Out(gpio.High)
		return err
	}
	return d.cs.Out(gpio.High)
}

// addr: 24 bit
// data: max 256 bytes
func (d *Device) programFlash(addr int, data []byte) error {
	const max24 = 1<<24 - 1 // 0xFFFFFF
	if addr < 0 || addr > max24 {
		return fmt.Errorf("address 0x%X out of 24-bit range", addr)
	}
	if len(data) > 256 {
		return errors.New("data must not exceed 256 bytes")
	}
	buf := make([]byte, 4+len(data))
	buf[0] = flashCmdPageProgram
	buf[1] = byte(addr >> 16)
	buf[2] = byte(addr >> 8)
	buf[3] = byte(addr)
	copy(buf[4:], data)

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
	time.Sleep(3 * time.Millisecond) // [W25Q128JV-DTR|9.6 AC Electrical Characteristics: tPP]
	return nil
}

func (d *Device) WriteFlash(r io.Reader) error {
	buf := [256]byte{}
	addr := 0
	for {
		if err := d.writeEnable(); err != nil {
			return err
		}

		n, err := r.Read(buf[:])
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		if err := d.programFlash(addr, buf[:n]); err != nil {
			return err
		}
		addr += n
	}
	return nil
}
