package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gentam/gice"
)

func packCommand(args []string) {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	var (
		inFilePath   string
		outFilePath  string
		skipBRAMInit bool
	)
	fs.StringVar(&inFilePath, "i", "", "input file (default stdin)")
	fs.StringVar(&outFilePath, "o", "", `output file (default: <input file>.bin; stdin → "out.bin")`)
	fs.BoolVar(&skipBRAMInit, "n", false, "skip initializing BRAM")
	fs.Parse(args)

	inFile := os.Stdin
	if inFilePath != "" {
		var err error
		inFile, err = os.Open(inFilePath)
		if err != nil {
			fatalf("open %q: %v", inFilePath, err)
		}
		defer inFile.Close()
	}

	if outFilePath == "" {
		if inFilePath == "" {
			outFilePath = "out.bin"
		} else {
			inFile := filepath.Base(inFilePath)
			outFilePath = strings.TrimSuffix(inFile, ".asc") + ".bin"
		}
	}
	outFile, err := os.Create(outFilePath)
	if err != nil {
		fatalf("create %q: %v", outFilePath, err)
	}
	defer outFile.Close()

	if err := gice.Pack(outFile, inFile); err != nil {
		fatalf("pack: %v", err)
	}
	fmt.Println(outFilePath)
}
