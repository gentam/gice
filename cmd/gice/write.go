package main

import (
	"flag"
	"os"

	"github.com/gentam/gice"
)

func writeCommand(args []string) {
	fs := flag.NewFlagSet("write", flag.ExitOnError)
	var (
		bulkErase bool
	)
	fs.BoolVar(&bulkErase, "e", false, "bulk erase entire flash")
	fs.Parse(args)

	if fs.NArg() == 0 && !bulkErase {
		fatalUsage("input file is required")
	}
	filename := fs.Arg(0)

	var file *os.File
	var err error
	if filename != "" {
		file, err = os.Open(filename)
		if err != nil {
			fatalf("failed to open file: %v", err)
		}
		defer file.Close()
	}

	d, err := gice.NewDevice()
	if err != nil {
		fatalf("%v", err)
	}

	d.HoldFPGAReset()
	defer d.ReleaseFPGAReset()

	if err := d.Flash.PowerUp(); err != nil {
		fatalf("flash power up failed: %v", err)
	}
	defer d.Flash.PowerDown()

	if err := d.Flash.LoadParams(); err != nil {
		fatalf("failed to load flash parameters: %v", err)
	}

	if bulkErase {
		if err := d.Flash.EraseChip(); err != nil {
			fatalf("erase chip failed: %v", err)
		}
	} else {
		stat, err := file.Stat()
		if err != nil {
			fatalf("failed to get file size: %v", err)
		}
		if err := d.Flash.Erase(0, int(stat.Size())); err != nil {
			fatalf("erase flash failed: %v", err)
		}
	}

	if file != nil {
		if err := d.Flash.Write(file); err != nil {
			fatalf("write flash failed: %v", err)
		}
	}
}
