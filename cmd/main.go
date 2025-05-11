package main

import (
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

	// [EB82 Appendix A. Sheet 2 of 5 ("USB to SPI/RS232")]
	// ADBUS0 | 16 | iCE_SCK
	// ADBUS1 | 17 | iCE_SI
	// ADBUS2 | 18 | iCE_SO
	// ADBUS4 | 21 | iCE_SS_B (CS)
	// ADBUS6 | 23 | iCE_CDONE
	// ADBUS7 | 24 | iCE_RESET

	cs := ft.D4 // ADBUS4 (GPIOLO → CS)
	jedecID, err := readJEDECID(cs, conn)
	if err != nil {
		fmt.Println("failed to read JEDEC ID:", err)
		return
	}
	jedecMicronN25Q032 := []byte{0x20, 0xBA, 0x16}
	if slices.Equal(jedecID[:3], jedecMicronN25Q032) {
		fmt.Printf("JEDEC ID matches Micron 32Mbit N25Q032 SPI flash (%X)\n", jedecID[:3])
		return
	}
	fmt.Printf("JEDEC ID: %X\n", jedecID)
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

func readJEDECID(cs gpio.PinOut, conn spi.Conn) (id []byte, err error) {
	buf := make([]byte, 4) // command + 3 bytes ID
	const cmdReadJDECID = 0x9F
	buf[0] = cmdReadJDECID

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
