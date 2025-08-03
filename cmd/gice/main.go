package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/host/v3"
	"periph.io/x/host/v3/ftdi"
)

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

func fatalUsage(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(2)
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage:
	gice <command> [arguments]

Commands:
	read	 read flash memory
	write	 write flash memory
`)
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() == 0 {
		usage()
	}

	switch cmd := flag.Arg(0); cmd {
	case "read":
		readCmd(flag.Args()[1:])
	case "write":
		writeCmd(flag.Args()[1:])
	case "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n", cmd)
		usage()
	}
}

func openFT2232H() (*ftdi.FT232H, error) {
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("host initialization failed: %w", err)
	}

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
			return ft, nil
		}
	}

	return nil, errors.New("not found")
}

func connectSPI() (spi.Conn, gpio.PinIO, error) {
	ft, err := openFT2232H()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open FT2232H device: %w", err)
	}

	sp, err := ft.SPI()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get SPI port: %w", err)
	}
	defer sp.Close()

	const clk = 30 * physic.MegaHertz // [AS_135 3.2.1 Divisors] specifies range in [92Hz, 30MHz], but periph.io's minimum is 100Hz
	mode := spi.Mode0                 // Mode0 and Mode3 are supported [n25q_32mb_3v_65nm.pdf|Table 7: SPI Modes]
	conn, err := sp.Connect(clk, mode, 8)
	if err != nil {
		return nil, nil, err
	}

	// [EB82|Appendix A. Sheet 2 of 5 (USB to SPI/RS232)]
	// ADBUS0 | 16 | iCE_SCK
	// ADBUS1 | 17 | iCE_SI
	// ADBUS2 | 18 | iCE_SO
	// ADBUS4 | 21 | iCE_SS_B (CS)
	// ADBUS6 | 23 | iCE_CDONE
	// ADBUS7 | 24 | iCE_RESET
	cs := ft.D4 // ADBUS4 (GPIOLO → CS)

	return conn, cs, nil
}
