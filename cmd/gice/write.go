package main

import (
	"flag"
	"fmt"
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
			fatalf("open %q: %v", filename, err)
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
		fatalf("flash power up: %v", err)
	}
	defer d.Flash.PowerDown()

	flashID, name, err := d.Flash.ReadID()
	if err != nil {
		fatalf("read flash ID: %v", err)
	}
	if name == "" {
		fmt.Fprintf(os.Stderr, "unknown flash ID (%X)\n", flashID)
	}

	if bulkErase {
		if err := d.Flash.EraseChip(); err != nil {
			fatalf("erase chip: %v", err)
		}
	} else {
		stat, err := file.Stat()
		if err != nil {
			fatalf("file stat: %v", err)
		}
		if err := d.Flash.Erase(0, int(stat.Size())); err != nil {
			fatalf("erase flash: %v", err)
		}
	}

	if file != nil {
		if err := d.Flash.Write(file); err != nil {
			fatalf("write flash: %v", err)
		}
	}
}
