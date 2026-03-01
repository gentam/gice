package gice

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Packer struct {
	device *fpgaDevice

	comment   []byte
	freqRange freqRange
	NoSleep   bool
	warmBoot  bool

	cram         [][][]bool // bank,x,y
	bram         [][][]bool // bank,x,y
	SkipBRAMInit bool
}

func (p *Packer) Pack(w io.Writer, r io.Reader) error {
	if err := p.ReadASCII(r); err != nil {
		return err
	}
	if err := p.WriteBits(w); err != nil {
		return err
	}
	return nil
}

func (p *Packer) Unpack(w io.Writer, r io.Reader) error {
	if err := p.ReadBits(r); err != nil {
		return err
	}
	if err := p.WriteASCII(w); err != nil {
		return err
	}
	return nil
}

type freqRange byte

const (
	freqRangeLow    freqRange = 0x00
	freqRangeMedium freqRange = 0x01
	freqRangeHigh   freqRange = 0x02
)

var hexMap = [256]int{
	'0': 0, '1': 1, '2': 2, '3': 3, '4': 4, '5': 5, '6': 6, '7': 7, '8': 8, '9': 9,
	'a': 10, 'b': 11, 'c': 12, 'd': 13, 'e': 14, 'f': 15,
	'A': 10, 'B': 11, 'C': 12, 'D': 13, 'E': 14, 'F': 15,
}

func (p *Packer) ReadASCII(r io.Reader) error {
	p.freqRange = freqRangeLow
	p.warmBoot = true

	scanner := bufio.NewScanner(r)
	var line string
	reuseLine := false
	for reuseLine || scanner.Scan() {
		if !reuseLine {
			line = scanner.Text()
		}
		reuseLine = false
		if line == "" {
			continue
		}
		cmd, rest, found := strings.Cut(line, " ")
		if !found {
			continue
		}

		switch cmd {
		case ".comment":
			// icepack ignores same line comment:
			// p.comment = append(p.comment, rest...)
			for scanner.Scan() {
				line = scanner.Text()
				if line != "" && line[0] == '.' {
					reuseLine = true
					break
				}

				p.comment = append(p.comment, line...)
				p.comment = append(p.comment, '\n')
			}

		case ".device":
			deviceName := string(rest)
			d := getFPGADevice(deviceName)
			p.device = d
			if p.device == nil {
				return fmt.Errorf("unsupported device: %q", deviceName)
			}

			p.cram = make([][][]bool, 4)
			for i := range len(p.cram) {
				p.cram[i] = make([][]bool, d.cramWidth)
				for x := range d.cramWidth {
					height := d.cramHeight
					if d.kind == ice5K && i%2 == 1 {
						height = d.cramHeight/2 + 8
					}
					p.cram[i][x] = make([]bool, height)
				}
			}

			p.bram = make([][][]bool, 4)
			for i := range len(p.bram) {
				width := d.bramWidth
				if d.kind == ice5K && i%2 == 1 {
					width = d.bramWidth / 2
				}
				p.bram[i] = make([][]bool, width)
				for y := range width {
					p.bram[i][y] = make([]bool, d.bramHeight)
				}
			}

		case ".warmboot":
			switch string(rest) {
			case "enabled":
				p.warmBoot = true
			case "disabled":
				p.warmBoot = false
			default:
				return fmt.Errorf("invalid warmboot value: %q", rest)
			}

		case ".io_tile", ".logic_tile", ".ramb_tile", ".ramt_tile", ".ipcon_tile",
			".dsp0_tile", ".dsp1_tile", ".dsp2_tile", ".dsp3_tile":
			if p.device == nil {
				return fmt.Errorf("missing .device before %s", cmd)
			}

			tileX, tileY, err := parseTileCoord(rest)
			if err != nil {
				return err
			}

			mapper := p.newCRAMTileMapper(tileX, tileY)
			tileKind := mapper.tileKind
			tileWidth := mapper.tileWidth

			if cmd != "."+string(tileKind)+"_tile" {
				return fmt.Errorf("got %s for %s tile (%d, %d)", cmd, tileKind, tileX, tileY)
			}

			for bitY := 0; bitY < 16 && scanner.Scan(); bitY++ {
				line = scanner.Text()
				if line != "" && line[0] == '.' {
					reuseLine = true
					break
				}

				for bitX := 0; bitX < len(line) && bitX < tileWidth; bitX++ {
					if line[bitX] == '1' {
						bit := mapper.bitToCRAM(bitX, bitY)
						p.cram[bit.bank][bit.x][bit.y] = true
					}
				}
			}

		case ".ram_data":
			if p.device == nil {
				return fmt.Errorf("missing .device before %s", cmd)
			}
			tileX, tileY, err := parseTileCoord(rest)
			if err != nil {
				return err
			}

			mapper := p.newBRAMTileMapper(tileX, tileY)
			for bitY := 0; bitY < 16 && scanner.Scan(); bitY++ {
				line = scanner.Text()
				if line != "" && line[0] == '.' {
					reuseLine = true
					break
				}

				for bitX, chIdx := 256-4, 0; chIdx < len(line) && bitX >= 0; bitX, chIdx = bitX-4, chIdx+1 {
					ch := line[chIdx]
					value := hexMap[ch]
					if value == 0 && ch != '0' {
						return fmt.Errorf("invalid hex value: %q", ch)
					}

					for i := range 4 {
						if (value & (1 << i)) != 0 {
							bit := mapper.nibbleBitToBRAM(bitX, bitY, i)
							p.bram[bit.bank][bit.x][bit.y] = true
						}
					}
				}
			}

		case ".extra_bit":
			if p.device == nil {
				return fmt.Errorf("missing .device before %s", cmd)
			}
			ss := strings.SplitN(rest, " ", 3)
			cramBank, err := strconv.Atoi(ss[0])
			if err != nil {
				return fmt.Errorf("invalid cram bank: %q", ss[0])
			}
			cramX, err := strconv.Atoi(ss[1])
			if err != nil {
				return fmt.Errorf("invalid cram x coordinate: %q", ss[1])
			}
			cramY, err := strconv.Atoi(ss[2])
			if err != nil {
				return fmt.Errorf("invalid cram y coordinate: %q", ss[2])
			}
			p.cram[cramBank][cramX][cramY] = true

		case ".sym":
			continue

		default:
			if cmd[0] == '.' {
				return fmt.Errorf("unknown command: %q", cmd)
			}
			return fmt.Errorf("unexpected data line: %q", line)
		}
	}
	return scanner.Err()
}

func parseTileCoord(rest string) (int, int, error) {
	fields := strings.Fields(rest)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("invalid coordinate: %q", rest)
	}

	tileX, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid X coordinate: %q", fields[0])
	}
	tileY, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid Y coordinate: %q", fields[1])
	}

	return tileX, tileY, nil
}

// [bitstream-format]
func (p *Packer) WriteBits(w io.Writer) error {
	cw := newCRCWriter(w)
	cw.write(0xFF, 0x00) // comment start
	for _, ch := range p.comment {
		if ch == '\n' {
			cw.write(0)
		} else {
			cw.write(ch)
		}
	}
	cw.write(0x00, 0xFF) // comment end

	// preamble
	cw.write(0x7E, 0xAA, 0x99, 0x7E)

	// 5: set internal oscillator frequency range
	cw.write(0x51)
	switch p.freqRange {
	case freqRangeLow:
		cw.write(0x00)
	case freqRangeMedium:
		cw.write(0x01)
	case freqRangeHigh:
		cw.write(0x02)
	default:
		return fmt.Errorf("unknown frequency range: %q", p.freqRange)
	}

	// 01: write CRAM Data, 05: reset CRC
	cw.write(0x01, 0x05)
	cw.resetCRC()

	// https://github.com/YosysHQ/icestorm/pull/113
	noSleepFlag := uint8(0)
	cw.write(0x92, 0x00) // disable warm boot
	if p.NoSleep {
		noSleepFlag = 1
	}
	if p.warmBoot {
		cw.write(0x20 | noSleepFlag)
	} else {
		cw.write(0x00 | noSleepFlag)
	}

	// 6: set bank width (16-bits, MSB first)
	cw.write(0x62)
	width := p.device.cramWidth - 1
	cw.write(uint8(width >> 8))
	cw.write(uint8(width))
	if p.device.kind != ice5K {
		cw.write(0x72) // 7: set bank height
		height := p.device.cramHeight
		cw.write(uint8(height >> 8))
		cw.write(uint8(height))
	}

	// 8: set bank offset (16-bits, MSB first)
	cw.write(0x82, 0x00, 0x00)
	for cramBank := range 4 {
		cramBits := []bool{}
		height := p.device.cramHeight
		if p.device.kind == ice5K && cramBank%2 == 1 {
			height = height/2 + 8
		}
		for cramY := range height {
			for cramX := range p.device.cramWidth {
				cramBits = append(cramBits, p.cram[cramBank][cramX][cramY])
			}
		}

		if p.device.kind == ice5K {
			cw.write(0x72) // set bank height
			cw.write(uint8(height >> 8))
			cw.write(uint8(height))
		}

		// 1: set bank number
		cw.write(0x11, uint8(cramBank))

		// 01: write CRAM Data
		cw.write(0x01, 0x01)
		for i := 0; i < len(cramBits); i += 8 {
			b := uint8(0)
			for j := range 8 {
				b <<= 1
				if cramBits[i+j] {
					b |= 1
				}
			}
			cw.write(b)
		}
		cw.write(0x00, 0x00) // end marker
	}

	bramChunkSize := 128
	if p.device.bramWidth > 0 && p.device.bramHeight > 0 {
		if p.device.kind != ice5K {
			cw.write(0x62) // set bank width
			width := p.device.bramWidth - 1
			cw.write(uint8(width >> 8))
			cw.write(uint8(width))
		}

		cw.write(0x72)
		cw.write(uint8(bramChunkSize >> 8))
		cw.write(uint8(bramChunkSize))

		for bramBank := range 4 {
			cw.write(0x11, uint8(bramBank))
			for offset := 0; offset < p.device.bramHeight; offset += bramChunkSize {
				bramBits := []bool{}
				width := p.device.bramWidth
				if p.device.kind == ice5K && bramBank%2 == 1 {
					width /= 2
				}

				for bramY := range bramChunkSize {
					for bramX := range width {
						bramBits = append(bramBits, p.bram[bramBank][bramX][bramY+offset])
					}
				}

				cw.write(0x82)
				cw.write(uint8(offset >> 8))
				cw.write(uint8(offset))

				if p.device.kind == ice5K {
					cw.write(0x62)
					cw.write(uint8((width - 1) >> 8))
					cw.write(uint8(width - 1))
				}

				if !p.SkipBRAMInit {
					cw.write(0x01, 0x03) // 03: write BRAM Data
					for i := 0; i < len(bramBits); i += 8 {
						b := uint8(0)
						for j := range 8 {
							b <<= 1
							if bramBits[i+j] {
								b |= 1
							}
						}
						cw.write(b)
					}
					cw.write(0x00, 0x00)
				}
			}
		}
	}

	// 2: CRC check
	cw.write(0x22)
	crc := cw.crc
	cw.write(uint8(crc >> 8))
	cw.write(uint8(crc))

	// 6: wakeup
	cw.write(0x01, 0x06)

	cw.write(0x00) // padding
	return cw.err
}

func (p *Packer) ReadBits(r io.Reader) error {
	return nil
}

func (p *Packer) WriteASCII(w io.Writer) error {
	if p.device == nil {
		return fmt.Errorf("missing device information")
	}

	b := strings.Builder{}
	b.WriteString(".comment")
	for _, c := range p.comment {
		if c == 0 {
			b.WriteByte('\n')
		} else {
			b.WriteByte(c)
		}
	}

	b.WriteString("\n.device ")
	b.WriteString(string(p.device.kind))
	b.WriteByte('\n')
	if !p.warmBoot {
		b.WriteString(".warmboot disabled\n")
	}

	// skip nosleep

	type tileBit struct {
		bank int
		x    int
		y    int
	}
	tileBits := map[tileBit]struct{}{}

	for y := range p.device.chipHeight + 2 { // +2 for IO tiles
		for x := range p.device.chipWidth + 2 {
			mapper := p.newCRAMTileMapper(x, y)
			tileKind := mapper.tileKind
			switch tileKind {
			case tileCorner, tileUnsupported:
				continue
			}

			tileWidth := mapper.tileWidth

			b.WriteByte('.')
			b.WriteString(string(tileKind))
			b.WriteString("_tile ")
			b.WriteString(strconv.Itoa(x))
			b.WriteByte(' ')
			b.WriteString(strconv.Itoa(y))
			b.WriteByte('\n')

			for bitY := range 16 {
				for bitX := range tileWidth {
					bit := mapper.bitToCRAM(bitX, bitY)
					tileBits[tileBit{bit.bank, bit.x, bit.y}] = struct{}{}
					if bit.x >= len(p.cram[bit.bank]) {
						return fmt.Errorf("cramX %d (bit %d, %d) exceeds bank size %d", bit.x, bitX, bitY, len(p.cram[bit.bank]))
					}
					if bit.y >= len(p.cram[bit.bank][bit.x]) {
						return fmt.Errorf("cramY %d (bit %d, %d) exceeds bank %d size %d", bit.y, bitX, bitY, bit.bank, len(p.cram[bit.bank][bit.x]))
					}
					if p.cram[bit.bank][bit.x][bit.y] {
						b.WriteByte('1')
					} else {
						b.WriteByte('0')
					}
				}
				b.WriteByte('\n')
			}

			if tileKind == tileRAMB && len(p.bram) != 0 {
				b.WriteString(".ram_data ")
				b.WriteString(strconv.Itoa(x))
				b.WriteByte(' ')
				b.WriteString(strconv.Itoa(y))
				b.WriteByte('\n')

				mapper := p.newBRAMTileMapper(x, y)
				for bitY := range 16 {
					for bitX := 256 - 4; bitX >= 0; bitX -= 4 {
						value := 0
						for i := range 4 {
							bit := mapper.nibbleBitToBRAM(bitX, bitY, i)
							if bit.x >= len(p.bram[bit.bank]) {
								return fmt.Errorf("bramX %d (bit %d, %d) exceeds BRAM size %d", bit.x, bitX+i, bitY, len(p.bram[bit.bank]))
							}
							if bit.y >= len(p.bram[bit.bank][bit.x]) {
								return fmt.Errorf("bramY %d (bit %d, %d) exceeds BRAM size %d", bit.y, bitX+i, bitY, len(p.bram[bit.bank][bit.x]))
							}
							if p.bram[bit.bank][bit.x][bit.y] {
								value += 1 << i
							}
						}
						b.WriteByte("0123456789abcdef"[value])
					}
					b.WriteByte('\n')
				}
			}
		}
	}

	for i := range 4 {
		for x := range p.device.cramWidth {
			for y := range p.device.cramHeight {
				_, ok := tileBits[tileBit{i, x, y}]
				if p.cram[i][x][y] && !ok {
					b.WriteString(".extra_bit ")
					b.WriteString(strconv.Itoa(i))
					b.WriteByte(' ')
					b.WriteString(strconv.Itoa(x))
					b.WriteByte(' ')
					b.WriteString(strconv.Itoa(y))
					b.WriteByte('\n')
				}
			}
		}
	}

	_, err := w.Write([]byte(b.String()))
	return err
}

//
// CRAM/BRAM tile mapping
//

var (
	ioTopBottomPermX = [18]int{23, 25, 26, 27, 16, 17, 18, 19, 20, 14, 32, 33, 34, 35, 36, 37, 4, 5}
	ioTopBottomPermY = [16]int{0, 1, 3, 2, 4, 5, 7, 6, 8, 9, 11, 10, 12, 13, 15, 14}
)

type cramBit struct {
	bank int
	x    int
	y    int
}

type bramBit struct {
	bank int
	x    int
	y    int
}

type cramTileMapper struct {
	tileKind        tileKind
	tileWidth       int
	isLeftRightEdge bool
	isRightHalf     bool
	isTopHalf       bool
	bankNum         int
	bankXOff        int
	bankYOff        int
	columnWidth     int
}

type bramTileMapper struct {
	bankNum int
	bankOff int
}

func (p *Packer) newCRAMTileMapper(tileX, tileY int) cramTileMapper {
	tileKind := p.device.tileKind(tileX, tileY)
	tileWidth := p.device.tileWidth(tileKind)
	isLeftRightEdge := tileX == 0 || tileX == p.device.chipWidth+1
	isRightHalf := tileX > p.device.chipWidth/2
	isTopHalf := tileY > p.device.chipHeight/2
	if p.device.kind == ice5K {
		isTopHalf = tileY > (p.device.chipHeight * 2 / 3)
	}

	bankNum := 0
	if isTopHalf {
		bankNum |= 1
	}
	if isRightHalf {
		bankNum |= 2
	}

	bankTx := tileX
	if isRightHalf {
		bankTx = p.device.chipWidth + 1 - tileX
	}
	bankTy := tileY
	if isTopHalf {
		bankTy = p.device.chipHeight + 1 - tileY
	}

	bankXOff := 0
	for i := range bankTx {
		bankXOff += p.device.cols[i]
	}

	return cramTileMapper{
		tileKind:        tileKind,
		tileWidth:       tileWidth,
		isLeftRightEdge: isLeftRightEdge,
		isRightHalf:     isRightHalf,
		isTopHalf:       isTopHalf,
		bankNum:         bankNum,
		bankXOff:        bankXOff,
		bankYOff:        16 * bankTy,
		columnWidth:     p.device.cols[bankTx],
	}
}

func (m cramTileMapper) bitToCRAM(bitX, bitY int) cramBit {
	cramX := 0
	cramY := 0

	if m.tileKind == tileIO {
		if m.isLeftRightEdge {
			cramX = m.bankXOff + m.columnWidth - 1 - bitX

			if m.isTopHalf {
				cramY = m.bankYOff + 15 - bitY
			} else {
				cramY = m.bankYOff + bitY
			}
		} else {
			cramY = m.bankYOff + 15 - ioTopBottomPermY[bitY]
			if m.isRightHalf {
				cramX = m.bankXOff + m.columnWidth - 1 - ioTopBottomPermX[bitX]
			} else {
				cramX = m.bankXOff + ioTopBottomPermX[bitX]
			}
		}
	} else {
		if m.isRightHalf {
			cramX = m.bankXOff + m.columnWidth - 1 - bitX
		} else {
			cramX = m.bankXOff + bitX
		}

		if m.isTopHalf {
			cramY = m.bankYOff + (15 - bitY)
		} else {
			cramY = m.bankYOff + bitY
		}
	}

	return cramBit{
		bank: m.bankNum,
		x:    cramX,
		y:    cramY,
	}
}

func (p *Packer) newBRAMTileMapper(tileX, tileY int) bramTileMapper {
	bankNum := 0
	isRightHalf := tileX > p.device.chipWidth/2
	isTopHalf := tileY > p.device.chipHeight/2
	if p.device.kind == ice5K {
		isTopHalf = tileY > (2 * p.device.chipHeight / 3)
	}

	yOffset := tileY - 1
	if p.device.kind == ice5K {
		if isTopHalf {
			bankNum |= 1
			yOffset = tileY - (2 * p.device.chipHeight / 3)
		}
	} else if isTopHalf {
		bankNum |= 1
		yOffset = tileY - p.device.chipHeight/2
	}
	if isRightHalf {
		bankNum |= 2
	}

	return bramTileMapper{
		bankNum: bankNum,
		bankOff: 16 * (yOffset / 2),
	}
}

func (m bramTileMapper) nibbleBitToBRAM(bitX, bitY, nibbleBit int) bramBit {
	wordBit := bitX + nibbleBit
	index := 256*bitY + (16*(wordBit/16) + 15 - wordBit%16)
	return bramBit{
		bank: m.bankNum,
		x:    m.bankOff + index%16,
		y:    index / 16,
	}
}
