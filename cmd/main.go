package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"slices"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/host/v3"
	"periph.io/x/host/v3/ftdi"
)

func main() {
	var (
		nread   int
		jidOnly bool
		outFile string
	)
	flag.IntVar(&nread, "n", 256, "number of bytes to read")
	flag.BoolVar(&jidOnly, "jid", false, "only print JEDEC ID")
	flag.StringVar(&outFile, "o", "", "output file (default: hexdump)")
	flag.Parse()

	if _, err := host.Init(); err != nil {
		fmt.Println("host initialization failed:", err)
		return
	}

	ft := openFT2232H()
	if ft == nil {
		fmt.Println("FT2232H device not found")
		return
	}

	sp, err := ft.SPI()
	if err != nil {
		fmt.Println("failed to get SPI port:", err)
		return
	}

	const clk = 30 * physic.MegaHertz // [AS_135 3.2.1 Divisors] specifies range in [92Hz, 30MHz], but periph.io's minimum is 100Hz
	mode := spi.Mode0                 // Mode0 and Mode3 are supported [n25q_32mb_3v_65nm.pdf Table 7: SPI Modes]
	conn, err := sp.Connect(clk, mode, 8)
	if err != nil {
		fmt.Println("SPI connection failed:", err)
		return
	}

	// [EB82 Appendix A. Sheet 2 of 5 (USB to SPI/RS232)]
	// ADBUS0 | 16 | iCE_SCK
	// ADBUS1 | 17 | iCE_SI
	// ADBUS2 | 18 | iCE_SO
	// ADBUS4 | 21 | iCE_SS_B (CS)
	// ADBUS6 | 23 | iCE_CDONE
	// ADBUS7 | 24 | iCE_RESET

	cs := ft.D4 // ADBUS4 (GPIOLO → CS)

	if err := releasePowerDown(cs, conn); err != nil {
		fmt.Println("release power down failed:", err)
		return
	}

	jedecID, err := readJEDECID(cs, conn)
	if err != nil {
		fmt.Println("read JEDEC ID failed:", err)
		return
	}
	if jidOnly {
		fmt.Printf("%X\n", jedecID[:3])
		return
	}
	if !isKnownJEDECID(jedecID) {
		fmt.Fprintf(os.Stderr, "unknown JEDEC ID (%X)\n", jedecID[:3])
	}

	data, err := readFlash(conn, cs, 0, nread)
	if err != nil {
		fmt.Println("read flash failed", err)
		return
	}
	if outFile == "" {
		fmt.Println(hex.Dump(data))
		return
	}
	if err := os.WriteFile(outFile, data, 0644); err != nil {
		fmt.Println("write file failed:", err)
	}
}

func openFT2232H() *ftdi.FT232H {
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
			return ft
		}
	}
	return nil
}

// [n25q_32mb_3v_65nm.pdf Table 16: Command Set]
// [W25Q128JV-DTR 8.1.2 Instruction Set Table 1]
const (
	cmdReleasePowerDown = 0xAB
	cmdReadJEDECID      = 0x9F
	cmdRead             = 0x03
)

func releasePowerDown(cs gpio.PinOut, conn spi.Conn) error {
	buf := []byte{cmdReleasePowerDown, 0, 0, 0, 0}
	if err := cs.Out(gpio.Low); err != nil {
		return err
	}
	if err := conn.Tx(buf, buf); err != nil {
		cs.Out(gpio.High)
		return err
	}
	// [W25Q128JV-DTR 9.6 AC Electrical Characteristics: tRES1]
	time.Sleep(3 * time.Microsecond)
	return cs.Out(gpio.High)
}

var (
	jedecMicronN25Q032      = []byte{0x20, 0xBA, 0x16}
	jedecWinbondW25Q128JVIM = []byte{0xEF, 0x70, 0x18}
)

func isKnownJEDECID(jedecID []byte) bool {
	id := jedecID[:3]
	return slices.Equal(id, jedecMicronN25Q032) || slices.Equal(id, jedecWinbondW25Q128JVIM)
}

func readJEDECID(cs gpio.PinOut, conn spi.Conn) (id []byte, err error) {
	buf := []byte{cmdReadJEDECID, 0, 0, 0}
	if err = cs.Out(gpio.Low); err != nil {
		return
	}
	if err = conn.Tx(buf, buf); err != nil {
		cs.Out(gpio.High)
		return
	}
	return buf[1:], cs.Out(gpio.High)
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
