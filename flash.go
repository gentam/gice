package gice

import (
	"errors"
	"fmt"
	"io"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/spi"
)

type Flash struct {
	conn spi.Conn
	cs   gpio.PinIO
}

func NewFlash(d *Device) *Flash {
	return &Flash{
		conn: d.conn,
		cs:   d.cs,
	}
}

// Flash commands supported by both Micron N25Q032 and Winbond W25Q128JVI.
//   - [N25Q32|Table 16: Command Set]
//   - [W25Q128|8.1.2 Instruction Set Table 1]
const (
	flashCmdReleasePowerDown = 0xAB
	flashCmdPowerDown        = 0xB9
	flashCmdReadID           = 0x9F
	flashCmdRead             = 0x03
	flashCmdWriteEnable      = 0x06
	flashCmdPageProgram      = 0x02
	flashCmdSubsectorErase   = 0x20 // Sector Erase (4KB)
	flashCmdSectorErase      = 0xD8 // Block Erase (64KB)
	flashCmdBulkErase        = 0xC7 // Chip Erase
)

var knownFlashIDs = map[[3]byte]string{
	{0x20, 0xBA, 0x16}: "Micron N25Q 32Mb",
	{0xEF, 0x70, 0x18}: "Winbond W25Q 128Mb",
}

func (f *Flash) IsKnown(id [3]byte) (string, bool) {
	if name, ok := knownFlashIDs[id]; ok {
		return name, true
	}
	return "", false
}

// tx wraps SPI transaction with CS assertion.
func (f *Flash) tx(buf []byte) (err error) {
	if err = f.cs.Out(gpio.Low); err != nil {
		return err
	}
	defer func() {
		if csErr := f.cs.Out(gpio.High); csErr != nil && err == nil {
			err = csErr
		}
	}()
	err = f.conn.Tx(buf, buf)
	return
}

func (f *Flash) PowerUp() error {
	buf := []byte{flashCmdReleasePowerDown}
	if err := f.tx(buf); err != nil {
		return err
	}
	time.Sleep(3 * time.Microsecond) // [W25Q128|9.6 AC Electrical Characteristics] tRES1
	return nil
}

func (f *Flash) PowerDown() error {
	buf := []byte{flashCmdPowerDown}
	if err := f.tx(buf); err != nil {
		return err
	}
	time.Sleep(3 * time.Microsecond) // [W25Q128|9.6 AC Electrical Characteristics] tDP
	return nil
}

// ReadID reads the JEDEC ID from the flash chip.
// Extended device string is ignored.
func (f *Flash) ReadID() (id [3]byte, err error) {
	buf := make([]byte, 4)
	buf[0] = flashCmdReadID

	if err = f.tx(buf); err != nil {
		return
	}
	return [3]byte(buf[1:]), err
}

// Read splits the read operation into multiple transactions to avoid exceeding
// the maximum transaction size.
func (f *Flash) Read(addr, n int) ([]byte, error) {
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
		// buf[4:] dummy bytes

		if err := f.tx(buf); err != nil {
			return nil, err
		}

		copy(out[off:], buf[cmdBytes:])

		addr += chunk
		off += chunk
		remaining -= chunk
	}
	return out, nil
}

func (f *Flash) writeEnable() error {
	buf := []byte{flashCmdWriteEnable}
	return f.tx(buf)
}

// addr: 24 bit
// data: max 256 bytes
func (f *Flash) program(addr int, data []byte) error {
	if err := f.writeEnable(); err != nil {
		return err
	}

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

	if err := f.tx(buf); err != nil {
		return err
	}
	time.Sleep(3 * time.Millisecond) // [W25Q128|9.6 AC Electrical Characteristics] tPP
	return nil
}

func (f *Flash) Write(r io.Reader) error {
	buf := [256]byte{}
	addr := 0
	for {
		n, err := r.Read(buf[:])
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		if err := f.program(addr, buf[:n]); err != nil {
			return err
		}
		addr += n
	}
	return nil
}

// SubsectorErase erases a 4KB subsector.
func (f *Flash) SubsectorErase(addr int) error {
	if err := f.writeEnable(); err != nil {
		return err
	}

	buf := make([]byte, 4)
	buf[0] = flashCmdSubsectorErase
	buf[1] = byte(addr >> 16)
	buf[2] = byte(addr >> 8)
	buf[3] = byte(addr)

	if err := f.tx(buf); err != nil {
		return err
	}

	// [N25Q32|Table 38: AC Characteristics and Operating Conditions] tSSE: 0.8s
	// [W25Q128|9.6 AC Electrical Characteristics] tSE (4KB): 400ms
	time.Sleep(800 * time.Millisecond)
	return nil
}

// SectorErase erases a 64KB sector.
func (f *Flash) SectorErase(addr int) error {
	if err := f.writeEnable(); err != nil {
		return err
	}

	buf := make([]byte, 4)
	buf[0] = flashCmdSectorErase
	buf[1] = byte(addr >> 16)
	buf[2] = byte(addr >> 8)
	buf[3] = byte(addr)

	if err := f.tx(buf); err != nil {
		return err
	}

	// [N25Q32|Table 38: AC Characteristics and Operating Conditions] tSE: 3s
	// [W25Q128|9.6 AC Electrical Characteristics] tBE2 (64KB): 2000ms
	time.Sleep(3 * time.Second)
	return nil
}

// BulkErase erases the entire flash chip.
func (f *Flash) BulkErase() error {
	if err := f.writeEnable(); err != nil {
		return err
	}

	buf := []byte{flashCmdBulkErase}
	if err := f.tx(buf); err != nil {
		return err
	}

	// [N25Q32|Table 38: AC Characteristics and Operating Conditions] tBE: 60s
	// [W25Q128|9.6 AC Electrical Characteristics] tCE: 200s
	time.Sleep(200 * time.Second) // TODO: check status register to return early?
	return nil
}

// Erase erases the size bytes starting from baseAddr by repeatedly calling
// SectorErase and SubsectorErase.
func (f *Flash) Erase(baseAddr, size int) error {
	const (
		sectorSize    = 64 << 10 // 64KB
		subsectorSize = 4 << 10  // 4KB
	)

	remaining := size
	addr := baseAddr

	// Use 64KB sectors for as much as possible
	for remaining >= sectorSize {
		if err := f.SectorErase(addr); err != nil {
			return err
		}
		addr += sectorSize
		remaining -= sectorSize
	}

	// Use 4KB subsectors for the rest
	for remaining > 0 {
		if err := f.SubsectorErase(addr); err != nil {
			return err
		}
		addr += subsectorSize
		remaining -= subsectorSize
	}

	return nil
}
