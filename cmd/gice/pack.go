package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"

	"github.com/gentam/gice"
)

func packCommand(args []string) {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	var (
		outFilePath  string
		skipBRAMInit bool
		noSleep      bool
	)
	fs.StringVar(&outFilePath, "o", "", `output file (default: <input file>.bin; stdin → "out.bin")`)
	fs.BoolVar(&skipBRAMInit, "n", false, "skip initializing BRAM")
	fs.BoolVar(&noSleep, "s", false, "disable final deep-sleep SPI flash command")
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
		if inFile, err = os.Open(inFilePath); err != nil {
			fatalf("open %q: %v", inFilePath, err)
		}
		defer inFile.Close()
	}

	stdoutTTY, err := isTTY(os.Stdout)
	if err != nil {
		fatalf("stdout: %v", err)
	}
	if outFilePath == "" && stdoutTTY {
		if inFilePath != "" {
			inFilename := filepath.Base(inFilePath)
			outFilePath = strings.TrimSuffix(inFilename, ".asc") + ".bin"
		} else {
			outFilePath = "out.bin"
		}
	}
	outFile := os.Stdout
	if outFilePath != "" {
		outFile, err = os.Create(outFilePath)
		if err != nil {
			fatalf("create %q: %v", outFilePath, err)
		}
		defer outFile.Close()
	}

	p := gice.Packer{}
	p.SkipBRAMInit = skipBRAMInit
	p.NoSleep = noSleep
	if err := p.Pack(outFile, inFile); err != nil {
		fatalf("pack: %v", err)
	}
}
