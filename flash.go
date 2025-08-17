package gice

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/spi"
)

type Flash struct {
	conn spi.Conn
	cs   gpio.PinIO
	id   [3]byte // JEDEC ID of the flash chip
	pr   *flashParams
}

func NewFlash(d *Device) *Flash {
	return &Flash{
		conn: d.conn,
		cs:   d.cs,
	}
}

// Flash commands:
//   - [N25Q32|Table 16: Command Set]
//   - [W25Q128|8.1.2 Instruction Set Table 1]
const (
	flashCmdPowerUp            = 0xAB // Release Power Down
	flashCmdPowerDown          = 0xB9
	flashCmdReadID             = 0x9F
	flashCmdRead               = 0x03
	flashCmdWriteEnable        = 0x06
	flashCmdPageProgram        = 0x02
	flashCmdErase4KB           = 0x20 // Subsector Erase / Sector Erase (4KB)
	flashCmdErase64KB          = 0xD8 // Sector Erase / Block Erase (64KB)
	flashCmdEraseChip          = 0xC7 // Bulk Erase / Chip Erase
	flashCmdReadStatusRegister = 0x05
)

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
	buf := []byte{flashCmdPowerUp}
	if err := f.tx(buf); err != nil {
		return err
	}
	time.Sleep(f.tRES1())
	return nil
}

func (f *Flash) PowerDown() error {
	buf := []byte{flashCmdPowerDown}
	if err := f.tx(buf); err != nil {
		return err
	}
	time.Sleep(f.tDP())
	return nil
}

// ReadID returns the JEDEC ID of the flash chip and configures its parameters.
// It returns a non-empty name for known IDs. The extended device string is ignored.
func (f *Flash) ReadID() (id [3]byte, name string, err error) {
	buf := make([]byte, 4)
	buf[0] = flashCmdReadID

	if err = f.tx(buf); err != nil {
		return
	}

	f.id = [3]byte(buf[1:])
	if params, ok := knownFlash[f.id]; ok {
		f.pr = &params
		name = params.name
	}
	return f.id, name, err
}

// Read performs a read operation, splitting it into multiple transactions if needed
// to stay within the maximum transaction size.
func (f *Flash) Read(addr, n int) ([]byte, error) {
	const (
		maxTx    = 65536 // [FTDI-AN_108]
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
func (f *Flash) pageProgram(addr int, data []byte) error {
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
	return f.BusyWait(100*time.Microsecond, f.tPP())
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
		if err := f.pageProgram(addr, buf[:n]); err != nil {
			return err
		}
		addr += n
	}
	return nil
}

func (f *Flash) Erase4KB(addr int) error {
	if err := f.writeEnable(); err != nil {
		return err
	}

	buf := make([]byte, 4)
	buf[0] = flashCmdErase4KB
	buf[1] = byte(addr >> 16)
	buf[2] = byte(addr >> 8)
	buf[3] = byte(addr)

	if err := f.tx(buf); err != nil {
		return err
	}
	return f.BusyWait(50*time.Millisecond, f.tErase4KB())
}

// Erase64KB erases a 64KB sector.
func (f *Flash) Erase64KB(addr int) error {
	if err := f.writeEnable(); err != nil {
		return err
	}

	buf := make([]byte, 4)
	buf[0] = flashCmdErase64KB
	buf[1] = byte(addr >> 16)
	buf[2] = byte(addr >> 8)
	buf[3] = byte(addr)

	if err := f.tx(buf); err != nil {
		return err
	}
	return f.BusyWait(100*time.Millisecond, f.tErase64KB())
}

// EraseChip bulk erase the entire chip.
func (f *Flash) EraseChip() error {
	if err := f.writeEnable(); err != nil {
		return err
	}

	buf := []byte{flashCmdEraseChip}
	if err := f.tx(buf); err != nil {
		return err
	}
	return f.BusyWait(time.Second, f.tEraseChip())
}

// Erase erases the size bytes starting from baseAddr by repeatedly calling
// Erase64KB and Erase4KB.
func (f *Flash) Erase(baseAddr, size int) error {
	const (
		sectorSize    = 64 << 10 // 64KB
		subsectorSize = 4 << 10  // 4KB
	)

	remaining := size
	addr := baseAddr

	// Use 64KB sectors for as much as possible
	for remaining >= sectorSize {
		if err := f.Erase64KB(addr); err != nil {
			return err
		}
		addr += sectorSize
		remaining -= sectorSize
	}

	// Use 4KB subsectors for the rest
	for remaining > 0 {
		if err := f.Erase4KB(addr); err != nil {
			return err
		}
		addr += subsectorSize
		remaining -= subsectorSize
	}

	return nil
}

// BusyWait waits for the flash to become ready by polling the status register's
// bit 0 with specified intervals, or until the timeout expires. Set timeout to
// 0 to wait indefinitely.
func (f *Flash) BusyWait(interval, timeout time.Duration) error {
	// Fast path
	if sr, err := f.ReadStatusRegister(); err == nil && !sr.Busy() {
		return nil
	}

	timer := time.NewTimer(timeout)
	if timeout == 0 {
		timer.Stop() // disable timer for unconfigured timeout
	}
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-timer.C:
			return nil
		case <-ticker.C:
			sr, err := f.ReadStatusRegister()
			if err != nil {
				return err
			}
			if !sr.Busy() {
				return nil
			}
		}
	}
}

// StatusRegister represents the status register of the flash chip.
//
//	Bits| [N25Q32|Table 9]                     | [W25Q128|7.1 Status Registers]
//	----+--------------------------------------+-------------------------------
//	7   | Status register write enable/disable | SRP: Status Register Protect
//	6   | Reserved                             | SEC: Sector protect
//	5   | Top/bottom                           | TB: Top/Bottom protect
//	4:2 | Block protect 2-0                    | BP2-0: Block Protect bit 2-0
//	1   | Write enable latch                   | WEL: Write Enable Latch
//	0   | Write in progress                    | BUSY: Erase/Write in progress
type StatusRegister byte

func (sr StatusRegister) StatusRegisterProtect() bool { return sr&(1<<7) != 0 }
func (sr StatusRegister) SectorProtect() bool         { return sr&(1<<6) != 0 }
func (sr StatusRegister) TopBottom() bool             { return sr&(1<<5) != 0 }
func (sr StatusRegister) BlockProtect2() bool         { return sr&(1<<4) != 0 }
func (sr StatusRegister) BlockProtect1() bool         { return sr&(1<<3) != 0 }
func (sr StatusRegister) BlockProtect0() bool         { return sr&(1<<2) != 0 }
func (sr StatusRegister) WriteEnabled() bool          { return sr&(1<<1) != 0 }
func (sr StatusRegister) Busy() bool                  { return sr&(1<<0) != 0 }

func (sr StatusRegister) String() string {
	b := fmt.Sprintf("%08b", byte(sr))
	s := []string{}
	if sr.StatusRegisterProtect() {
		s = append(s, "SRP")
	}
	if sr.SectorProtect() {
		s = append(s, "SEC")
	}
	if sr.TopBottom() {
		s = append(s, "TB")
	}
	if sr.BlockProtect2() {
		s = append(s, "BP2")
	}
	if sr.BlockProtect1() {
		s = append(s, "BP1")
	}
	if sr.BlockProtect0() {
		s = append(s, "BP0")
	}
	if sr.WriteEnabled() {
		s = append(s, "WEL")
	}
	if sr.Busy() {
		s = append(s, "BUSY")
	}
	if len(s) == 0 {
		return b
	}
	return b + " " + strings.Join(s, ",")
}

func (f *Flash) ReadStatusRegister() (StatusRegister, error) {
	buf := []byte{flashCmdReadStatusRegister, 0}
	if err := f.tx(buf); err != nil {
		return 0, err
	}
	return StatusRegister(buf[1]), nil
}
