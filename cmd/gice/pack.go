package main

import (
	"flag"
	"os"

	"github.com/gentam/gice"
)

func packCommand(args []string) {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	var (
		inFilename   string
		outFilename  string
		skipBRAMInit bool
	)
	fs.StringVar(&inFilename, "i", "", "input file (default stdin)")
	fs.StringVar(&outFilename, "o", "", `output file (default: <input file>.bin; stdin → "out.bin")`)
	fs.BoolVar(&skipBRAMInit, "n", false, "skip initializing BRAM")
	fs.Parse(args)

	inFile := os.Stdin
	if inFilename != "" {
		var err error
		inFile, err = os.Open(inFilename)
		if err != nil {
			fatalf("open %q: %v", inFilename, err)
		}
		defer inFile.Close()
	}

	if outFilename == "" {
		if inFilename == "" {
			outFilename = "out.bin"
		} else {
			outFilename = inFilename + ".bin"
		}
	}
	outFile, err := os.Create(outFilename)
	if err != nil {
		fatalf("create %q: %v", outFilename, err)
	}
	defer outFile.Close()

	if err := gice.Pack(inFile, outFile); err != nil {
		fatalf("pack: %v", err)
	}
}
