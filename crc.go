package gice

import "io"

const crcInit = 0xFFFF

// updateCRC updates the CRC-16-CCITT checksum.
func updateCRC(crc uint16, data byte) uint16 {
	for i := 7; i >= 0; i-- {
		if ((crc >> 15) ^ uint16((data>>i)&1)) != 0 {
			crc = (crc << 1) ^ 0x1021
		} else {
			crc <<= 1
		}
	}
	return crc
}

// crcWriter proxies writes to w while updating a CRC-16-CCITT checksum
// (init = 0xFFFF).
type crcWriter struct {
	w       io.Writer
	crc     uint16
	written int64
}

func newCRCWriter(w io.Writer) *crcWriter {
	return &crcWriter{
		w:   w,
		crc: crcInit,
	}
}

// resetCRC resets the CRC checksum to 0xFFFF.
func (cw *crcWriter) resetCRC() {
	cw.crc = crcInit
}

func (cw *crcWriter) Write(p []byte) (int, error) {
	return cw.write(p...)
}

func (cw *crcWriter) write(p ...byte) (int, error) {
	n, err := cw.w.Write(p)
	if err != nil {
		return n, err
	}

	cw.written += int64(n)
	for _, b := range p[:n] {
		cw.crc = updateCRC(cw.crc, b)
	}
	return n, nil
}

// crcReader proxies reads from r while updating a CRC-16-CCITT checksum
// (init = 0xFFFF).
type crcReader struct {
	r    io.ByteReader
	crc  uint16
	read int64
}

func newCRCReader(r io.ByteReader) *crcReader {
	return &crcReader{
		r:   r,
		crc: crcInit,
	}
}

// resetCRC resets the CRC checksum to 0xFFFF.
func (cr *crcReader) resetCRC() {
	cr.crc = crcInit
}

func (cr *crcReader) readByte() (byte, error) {
	b, err := cr.r.ReadByte()
	if err != nil {
		return 0, err
	}

	cr.read++
	cr.crc = updateCRC(cr.crc, b)
	return b, nil
}
