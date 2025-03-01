package main

import (
	"log"
	"os"
)

func main() {
	f, err := os.Open("/dev/cu.usbserial-2100")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	log.Println(f.Name())
}
