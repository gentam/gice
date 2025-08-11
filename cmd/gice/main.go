package main

import (
	"flag"
	"fmt"
	"os"
)

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

func fatalUsage(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(2)
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage:
	gice <command> [arguments]

Commands:
	read [-id] [-n size] [-o file]
		read flash memory

	write [-e] <file>
		write/erase flash memory

	info
		print device information
`)
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() == 0 {
		usage()
	}

	switch cmd := flag.Arg(0); cmd {
	case "read":
		readCommand(flag.Args()[1:])
	case "write":
		writeCommand(flag.Args()[1:])
	case "info":
		infoCommand()
	case "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n", cmd)
		usage()
	}
}
