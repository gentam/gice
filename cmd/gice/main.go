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
	read	read flash memory
	write	write/erase flash memory
	pack	convert ASCII input into a bitstream file
	unpack	convert bitstream input into an ASCII file
	info	print device information

Run "%s <command> -h" for more information about a command.
`, os.Args[0], os.Args[0])
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
	case "unpack":
		unpackCommand(rest)
	case "info":
		infoCommand()
	case "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n", cmd)
		usage()
	}
}

func isTTY(f *os.File) (bool, error) {
	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	return (info.Mode() & os.ModeCharDevice) != 0, nil
}
