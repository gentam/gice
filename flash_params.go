package gice

import "time"

type flashParams struct {
	name string

	tRES1      time.Duration
	tDP        time.Duration
	tPP        time.Duration
	tErase4KB  time.Duration
	tErase64KB time.Duration
	tEraseChip time.Duration
}

var (
	flashIDMicronN25Q32   = [3]byte{0x20, 0xBA, 0x16}
	flashIDWinbondW25Q128 = [3]byte{0xEF, 0x70, 0x18}
)

var knownFlash = map[[3]byte]flashParams{
	flashIDMicronN25Q32: {
		name: "Micron N25Q 32Mb",

		// [N25Q32|Table 38: AC Characteristics and Operating Conditions]
		// tPP: PAGE PROGRAM cycle time (256 bytes)
		tPP: time.Duration(5 * time.Millisecond),
		// tSSE: Subsector ERASE cycle time
		tErase4KB: time.Duration(800 * time.Millisecond),
		// tSE: Sector ERASE cycle time
		tErase64KB: time.Duration(3 * time.Second),
		// tBE: Bulk ERASE cycle time
		tEraseChip: time.Duration(60 * time.Second),
	},

	flashIDWinbondW25Q128: {
		name: "Winbond W25Q 128Mb",

		// [W25Q128|9.6 AC Electrical Characteristics]:
		// tRES1: /CS High to Standby Mode without ID Read
		tRES1: time.Duration(3 * time.Microsecond),
		// tDP: /CS High to Power-down Mode
		tDP: time.Duration(3 * time.Microsecond),
		// tPP: Page Program Time
		tPP: time.Duration(3 * time.Millisecond),
		// tSE: Sector Erase Time (4KB)
		tErase4KB: time.Duration(400 * time.Millisecond),
		// tBE2: Block Erase Time (64KB)
		tErase64KB: time.Duration(2000 * time.Millisecond),
		// tCE: Chip Erase Time
		tEraseChip: time.Duration(200 * time.Second),
	},
}

func (f *Flash) paramOrMax(get func(*flashParams) time.Duration) time.Duration {
	// get parameter if configured
	if f.pr != nil {
		return get(f.pr)
	}

	// fall back to maximum duration from all known flash parameters
	var tmax time.Duration
	for _, param := range knownFlash {
		tmax = max(tmax, get(&param))
	}
	return tmax
}

func (f *Flash) tRES1() time.Duration {
	return f.paramOrMax(func(p *flashParams) time.Duration { return p.tRES1 })
}
func (f *Flash) tDP() time.Duration {
	return f.paramOrMax(func(p *flashParams) time.Duration { return p.tDP })
}
func (f *Flash) tPP() time.Duration {
	return f.paramOrMax(func(p *flashParams) time.Duration { return p.tPP })
}
func (f *Flash) tErase4KB() time.Duration {
	return f.paramOrMax(func(p *flashParams) time.Duration { return p.tErase4KB })
}
func (f *Flash) tErase64KB() time.Duration {
	return f.paramOrMax(func(p *flashParams) time.Duration { return p.tErase64KB })
}
func (f *Flash) tEraseChip() time.Duration {
	return f.paramOrMax(func(p *flashParams) time.Duration { return p.tEraseChip })
}
