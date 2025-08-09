package main

import (
	"errors"
	"fmt"
	"io"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/spi"
)

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

func isKnownFlashID(id [3]byte) (string, bool) {
	if name, ok := knownFlashIDs[id]; ok {
		return name, true
	}
	return "", false
}

func readFlashID(conn spi.Conn, cs gpio.PinOut) (id [3]byte, err error) {
	buf := []byte{flashCmdReadID, 0, 0, 0}
	if err = cs.Out(gpio.Low); err != nil {
		return
	}
	if err = conn.Tx(buf, buf); err != nil {
		cs.Out(gpio.High)
		return
	}
	return [3]byte(buf[1:]), cs.Out(gpio.High)
}

// readFlash splits the read operation into multiple transactions to avoid
// exceeding the maximum transaction size.
func readFlash(conn spi.Conn, cs gpio.PinOut, addr, n int) ([]byte, error) {
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

		if err := cs.Out(gpio.Low); err != nil {
			return nil, err
		}
		txErr := conn.Tx(buf, buf)
		if csErr := cs.Out(gpio.High); csErr != nil {
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

func writeEnable(conn spi.Conn, cs gpio.PinOut) error {
	buf := []byte{flashCmdWriteEnable}
	if err := cs.Out(gpio.Low); err != nil {
		return err
	}
	if err := conn.Tx(buf, buf); err != nil {
		cs.Out(gpio.High)
		return err
	}
	return cs.Out(gpio.High)
}

// addr: 24 bit
// data: max 256 bytes
func programFlash(conn spi.Conn, cs gpio.PinOut, addr int, data []byte) error {
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

	if err := cs.Out(gpio.Low); err != nil {
		return err
	}
	if err := conn.Tx(buf, buf); err != nil {
		cs.Out(gpio.High)
		return err
	}
	if err := cs.Out(gpio.High); err != nil {
		return err
	}
	time.Sleep(3 * time.Millisecond) // [W25Q128JV-DTR|9.6 AC Electrical Characteristics: tPP]
	return nil
}

func writeFlash(conn spi.Conn, cs gpio.PinOut, r io.Reader) error {
	buf := [256]byte{}
	addr := 0
	for {
		if err := writeEnable(conn, cs); err != nil {
			return err
		}

		n, err := r.Read(buf[:])
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		if err := programFlash(conn, cs, addr, buf[:n]); err != nil {
			return err
		}
		addr += n
	}
	return nil
}
