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

	conn, cs, err := connectSPI()
	if err != nil {
		fatalf("SPI connection failed: %v", err)
	}

	if err := releasePowerDown(conn, cs); err != nil {
		fatalf("release power down failed: %v", err)
	}

	flashID, err := readFlashID(conn, cs)
	if err != nil {
		fatalf("read flash ID failed: %v", err)
	}
	name, known := isKnownFlashID(flashID)
	if idOnly {
		fmt.Printf("%X\t%s\n", flashID, name)
		return
	}
	if !known {
		fmt.Fprintf(os.Stderr, "unknown flash ID (%X)\n", flashID)
	}

	data, err := readFlash(conn, cs, 0, nread)
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
