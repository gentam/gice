package gice

import "io"

// crcWriter wraps an io.Writer with CRC update.
type crcWriter struct {
	w       io.Writer
	crc     uint16
	written int64
}

// newCRCWriter creates a new crcWriter with initial CRC value 0xFFFF.
func newCRCWriter(w io.Writer) *crcWriter {
	return &crcWriter{
		w:   w,
		crc: 0xFFFF,
	}
}

// resetCRC resets the CRC value to 0xFFFF.
func (cw *crcWriter) resetCRC() {
	cw.crc = 0xFFFF
}

func (cw *crcWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	if err != nil {
		return n, err
	}

	cw.written += int64(n)
	for _, b := range p[:n] {
		cw.updateCRC(b)
	}
	return n, nil
}

// updateCRC updates CRC-16-CCITT value.
func (cw *crcWriter) updateCRC(data byte) {
	for i := 7; i >= 0; i-- {
		if ((cw.crc >> 15) ^ uint16((data>>i)&1)) != 0 {
			cw.crc = (cw.crc << 1) ^ 0x1021
		} else {
			cw.crc <<= 1
		}
	}
}
