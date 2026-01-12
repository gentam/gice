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
	if err := fs.Parse(args); err != nil {
		fatalUsage("invalid arguments: %v", err)
	}

	stdinTTY, err := isTTY(os.Stdin)
	if err != nil {
		fatalf("stdin: %v", err)
	}
	inFilePath := fs.Arg(0)
	if inFilePath == "" && stdinTTY {
		fatalUsage("missing input")
	}

	inFile := os.Stdin
	if inFilePath != "" {
		inFile, err = os.Open(inFilePath)
		if err != nil {
			fatalf("open %q: %v", inFilePath, err)
		}
		defer inFile.Close()
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
		stat, err := inFile.Stat()
		if err != nil {
			fatalf("file stat: %v", err)
		}
		if err := d.Flash.Erase(0, int(stat.Size())); err != nil {
			fatalf("erase flash: %v", err)
		}
	}

	if inFile != nil {
		if err := d.Flash.Write(inFile); err != nil {
			fatalf("write flash: %v", err)
		}
	}
}
