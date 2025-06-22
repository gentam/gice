package main

import (
	"encoding/hex"
	"fmt"
	"slices"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/host/v3"
	"periph.io/x/host/v3/ftdi"
)

func main() {
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

	jedecID, err := readJEDECID(cs, conn)
	if err != nil {
		fmt.Println("read JEDEC ID failed:", err)
		return
	}
	jedecMicronN25Q032 := []byte{0x20, 0xBA, 0x16}
	if !slices.Equal(jedecID[:3], jedecMicronN25Q032) {
		fmt.Printf("JEDEC ID does not match Micron 32Mbit N25Q032 SPI flash (%X)\n", jedecID[:3])
		return
	}

	data, err := readFlash(conn, cs, 0, 256)
	if err != nil {
		fmt.Println("read flash failed", err)
		return
	}
	fmt.Println(hex.Dump(data))
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
const (
	cmdReadID = 0x9F
	cmdRead   = 0x03
)

func readJEDECID(cs gpio.PinOut, conn spi.Conn) (id []byte, err error) {
	buf := make([]byte, 4) // command + 3 bytes ID
	buf[0] = cmdReadID
	if err = cs.Out(gpio.Low); err != nil {
		return
	}
	if err = conn.Tx(buf, buf); err != nil {
		fmt.Println("SPI transaction failed:", err)
		return
	}
	if err = cs.Out(gpio.High); err != nil {
		return
	}
	return buf[1:], nil
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

		cs.Out(gpio.Low)
		if err := conn.Tx(buf, buf); err != nil {
			cs.Out(gpio.High)
			return nil, err
		}
		cs.Out(gpio.High)

		copy(out[off:], buf[cmdBytes:])

		addr += chunk
		off += chunk
		remaining -= chunk
	}
	return out, nil
}
