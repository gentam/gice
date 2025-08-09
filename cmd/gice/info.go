package main

import (
	"fmt"

	"periph.io/x/host/v3/ftdi"
)

func infoCommand() {
	dev, err := NewDevice()
	if err != nil {
		fatalf("failed create device: %v", err)
	}
	d := dev.ft

	// Reference: https://github.com/periph/cmd/tree/main/ftdi-list
	i := ftdi.Info{}
	d.Info(&i)
	fmt.Printf("Type:            %s\n", i.Type)
	fmt.Printf("Vendor ID:       %#04x\n", i.VenID)
	fmt.Printf("Device ID:       %#04x\n", i.DevID)

	ee := ftdi.EEPROM{}
	if err := d.EEPROM(&ee); err != nil {
		fatalf("failed to read EEPROM: %v", err)
	}

	fmt.Printf("Manufacturer:    %s\n", ee.Manufacturer)
	fmt.Printf("ManufacturerID:  %s\n", ee.ManufacturerID)
	fmt.Printf("Desc:            %s\n", ee.Desc)
	fmt.Printf("Serial:          %s\n", ee.Serial)

	h := ee.AsHeader()
	fmt.Printf("MaxPower:        %dmA\n", h.MaxPower)
	fmt.Printf("SelfPowered:	 %x\n", h.SelfPowered)
	fmt.Printf("RemoteWakeup:	 %x\n", h.RemoteWakeup)
	fmt.Printf("PullDownEnable:	 %x\n", h.PullDownEnable)

	for _, p := range d.Header() {
		fmt.Printf("%s: %s\n", p, p.Function())
	}
}
