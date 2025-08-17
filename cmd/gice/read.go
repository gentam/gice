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
		outFile    string
	)
	fs.IntVar(&nread, "n", 256, "number of bytes to read")
	fs.BoolVar(&idOnly, "id", false, "just print flash ID")
	fs.BoolVar(&statusOnly, "s", false, "just print flash status register")
	fs.StringVar(&outFile, "o", "", "output file (default: hexdump)")
	fs.Parse(args)

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

	if statusOnly {
		sr, err := d.Flash.ReadStatusRegister()
		if err != nil {
			fatalf("read flash status register failed: %v", err)
		}
		fmt.Println(sr)
		return
	}

	flashID, name, err := d.Flash.ReadID()
	if err != nil {
		fatalf("read flash ID failed: %v", err)
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
