package main

import (
	"fmt"
	"log"
	"os"
	"syscall"
	"unsafe"
)

func main() {
	path := "/dev/cu.usbserial-2101"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer syscall.Close(fd)

	term := getTerm(fd)
	term.Ispeed = syscall.B9600
	term.Ospeed = syscall.B9600
	term.Cc[syscall.VMIN] = 5
	term.Cc[syscall.VTIME] = 0
	setTerm(fd, term)

	file := os.NewFile(uintptr(fd), "usbserial")
	if file == nil {
		log.Fatal("Failed to create file")
	}
	defer file.Close()

	go func() {
		stdin := getTerm(syscall.Stdin)
		// Set to raw mode
		stdin.Lflag &^= syscall.ICANON | syscall.ECHO
		stdin.Cc[syscall.VMIN] = 1
		stdin.Cc[syscall.VTIME] = 0
		setTerm(syscall.Stdin, stdin)

		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				log.Println("Error reading from stdin:", err)
				break
			}
			file.Write(buf[:n])
		}
	}()

	buf := make([]byte, 8)
	for {
		n, err := file.Read(buf)
		if err != nil {
			log.Println("Error reading from file:", err)
			break
		}
		fmt.Printf("%s\r", string(buf[:n]))
	}
}

func getTerm(fd int) syscall.Termios {
	term := syscall.Termios{}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		syscall.TIOCGETA,
		uintptr(unsafe.Pointer(&term)),
	)
	if errno != 0 {
		log.Fatal("Error getting terminal attributes", errno)
	}
	return term
}

func setTerm(fd int, term syscall.Termios) {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		syscall.TIOCSETA,
		uintptr(unsafe.Pointer(&term)),
	)
	if errno != 0 {
		log.Fatal("Error setting terminal attributes", errno)
	}
}
