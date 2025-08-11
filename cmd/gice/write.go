package main

import (
	"flag"
	"os"
)

func writeCommand(args []string) {
	fs := flag.NewFlagSet("write", flag.ExitOnError)
	var (
		filename string
	)
	fs.StringVar(&filename, "f", "", "input file")
	fs.Parse(args)

	if filename == "" {
		fatalUsage("input file is required")
	}

	file, err := os.Open(filename)
	if err != nil {
		fatalf("failed to open file: %v", err)
	}
	defer file.Close()

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

	if err := d.WriteFlash(file); err != nil {
		fatalf("failed to write flash: %v", err)
	}
}
