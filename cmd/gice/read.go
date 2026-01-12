package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/gentam/gice"
)

func readCommand(args []string) {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	var (
		nread      int
		idOnly     bool
		statusOnly bool
	)
	fs.IntVar(&nread, "n", 256, "number of bytes to read")
	fs.BoolVar(&idOnly, "id", false, "just print flash ID")
	fs.BoolVar(&statusOnly, "s", false, "just print flash status register")
	if err := fs.Parse(args); err != nil {
		fatalUsage("invalid arguments: %v", err)
	}

	stdoutTTY, err := isTTY(os.Stdout)
	if err != nil {
		fatalf("stdout: %v", err)
	}
	outFile := os.Stdout
	if filename := fs.Arg(0); filename != "" {
		if outFile, err = os.Create(filename); err != nil {
			fatalf("create file: %v", err)
		}
		defer outFile.Close()
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

	if statusOnly {
		sr, err := d.Flash.ReadStatusRegister()
		if err != nil {
			fatalf("read flash status register: %v", err)
		}
		fmt.Println(sr)
		return
	}

	flashID, name, err := d.Flash.ReadID()
	if err != nil {
		fatalf("read flash ID: %v", err)
	}
	if idOnly {
		fmt.Printf("%X\t%s\n", flashID, name)
		return
	}
	if name == "" {
		fmt.Fprintf(os.Stderr, "unknown flash ID (%X)\n", flashID)
	}

	data, err := d.Flash.Read(0, nread)
	if err != nil {
		fatalf("read flash: %v", err)
	}

	if outFile == os.Stdout && stdoutTTY {
		fmt.Println(hex.Dump(data))
		return
	}
	if _, err := outFile.Write(data); err != nil {
		fatalf("write: %v", err)
	}
}
