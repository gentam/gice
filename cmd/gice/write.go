package main

import (
	"flag"
	"io"
	"os"

	"github.com/gentam/gice"
)

func writeCommand(args []string) {
	fs := flag.NewFlagSet("write", flag.ExitOnError)
	var (
		filename  string
		bulkErase bool
	)
	fs.StringVar(&filename, "f", "", "input file")
	fs.BoolVar(&bulkErase, "e", false, "bulk erase entire flash")
	fs.Parse(args)

	if filename == "" && !bulkErase {
		fatalUsage("input file is required")
	}

	var input io.ReadCloser
	var err error
	if filename != "" {
		input, err = os.Open(filename)
		if err != nil {
			fatalf("failed to open file: %v", err)
		}
		defer input.Close()
	}

	d, err := gice.NewDevice()
	if err != nil {
		fatalf("%v", err)
	}

	d.ResetFPGA(false) // prevent FPGA from acting as a SPI master
	defer d.ResetFPGA(true)

	if err := d.Flash.PowerUp(); err != nil {
		fatalf("flash power up failed: %v", err)
	}
	defer d.Flash.PowerDown()

	if bulkErase {
		if err := d.Flash.BulkErase(); err != nil {
			fatalf("bulk erase flash failed: %v", err)
		}
	} else {
		stat, err := input.(*os.File).Stat()
		if err != nil {
			fatalf("failed to get file size: %v", err)
		}
		if err := d.Flash.Erase(0, int(stat.Size())); err != nil {
			fatalf("erase failed: %v", err)
		}
	}

	if input != nil {
		if err := d.Flash.Write(input); err != nil {
			fatalf("write flash failed: %v", err)
		}
	}
}
