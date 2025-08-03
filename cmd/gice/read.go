package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/spi"
)

func readCmd(args []string) {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	var (
		nread   int
		idOnly  bool
		outFile string
	)
	fs.IntVar(&nread, "n", 256, "number of bytes to read")
	fs.BoolVar(&idOnly, "id", false, "only print flash ID and exit")
	fs.StringVar(&outFile, "o", "", "output file (default: hexdump)")
	fs.Parse(args)

	conn, cs, err := connectSPI()
	if err != nil {
		fatalf("SPI connection failed: %v", err)
	}

	if err := releasePowerDown(conn, cs); err != nil {
		fatalf("release power down failed: %v", err)
	}

	flashID, err := readFlashID(conn, cs)
	if err != nil {
		fatalf("read flash ID failed: %v", err)
	}
	name, known := isKnownFlashID(flashID)
	if idOnly {
		fmt.Printf("%X\t%s\n", flashID, name)
		return
	}
	if !known {
		fmt.Fprintf(os.Stderr, "unknown flash ID (%X)\n", flashID)
	}

	data, err := readFlash(conn, cs, 0, nread)
	if err != nil {
		fatalf("read flash failed: %v", err)
	}
	if outFile == "" {
		fmt.Println(hex.Dump(data))
		return
	}
	if err := os.WriteFile(outFile, data, 0644); err != nil {
		fmt.Fprintln(os.Stderr, "write file failed:", err)
	}
}

// [n25q_32mb_3v_65nm.pdf|Table 16: Command Set]
// [W25Q128JV-DTR|8.1.2 Instruction Set Table 1]
const (
	cmdReleasePowerDown = 0xAB
	cmdReadID           = 0x9F
	cmdRead             = 0x03
)

func releasePowerDown(conn spi.Conn, cs gpio.PinOut) error {
	buf := []byte{cmdReleasePowerDown, 0, 0, 0, 0}
	if err := cs.Out(gpio.Low); err != nil {
		return err
	}
	if err := conn.Tx(buf, buf); err != nil {
		cs.Out(gpio.High)
		return err
	}
	err := cs.Out(gpio.High)
	time.Sleep(3 * time.Microsecond) // [W25Q128JV-DTR|9.6 AC Electrical Characteristics: tRES1]
	return err
}

var (
	knownFlashIDs = map[[3]byte]string{
		{0x20, 0xBA, 0x16}: "Micron N25Q032",
		{0xEF, 0x70, 0x18}: "Winbond W25Q128JVIM",
	}
)

func isKnownFlashID(id [3]byte) (string, bool) {
	if name, ok := knownFlashIDs[id]; ok {
		return name, true
	}
	return "", false
}

func readFlashID(conn spi.Conn, cs gpio.PinOut) (id [3]byte, err error) {
	buf := []byte{cmdReadID, 0, 0, 0}
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
		buf[0] = cmdRead
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
