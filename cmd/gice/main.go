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
	%s <command> [arguments]

Commands:
	read [-id] [-n size] [-o file] [-s]
		read flash memory

	write [-e] <file>
		write/erase flash memory

	info
		print device information
`, os.Args[0])
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() == 0 {
		usage()
	}

	cmd := flag.Arg(0)
	rest := flag.Args()[1:]
	switch cmd {
	case "read":
		readCommand(rest)
	case "write":
		writeCommand(rest)
	case "pack":
		packCommand(rest)
	case "info":
		infoCommand()
	case "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n", cmd)
		usage()
	}
}
