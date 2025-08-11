package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
)

func readCommand(args []string) {
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

	d, err := NewDevice()
	if err != nil {
		fatalf("%v", err)
	}

	d.ResetFPGA(false) // prevent FPGA from acting as a SPI master
	defer d.ResetFPGA(true)

	if err := d.FlashPowerUp(); err != nil {
		fatalf("flash power up failed: %v", err)
	}
	defer d.FlashPowerDown()

	flashID, err := d.ReadFlashID()
	if err != nil {
		fatalf("read flash ID failed: %v", err)
	}
	name, known := d.IsKnownFlashID(flashID)
	if idOnly {
		fmt.Printf("%X\t%s\n", flashID, name)
		return
	}
	if !known {
		fmt.Fprintf(os.Stderr, "unknown flash ID (%X)\n", flashID)
	}

	data, err := d.ReadFlash(0, nread)
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
