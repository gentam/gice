package gice

type fpgaDevice struct {
	kind       deviceKind
	chipWidth  int
	chipHeight int
	cols       []int

	cramWidth, cramHeight int
	bramWidth, bramHeight int

	tileKind func(x, y int) tileKind
}

type deviceKind string

const (
	ice384  deviceKind = "384"
	ice1K   deviceKind = "1k"
	iceU4K  deviceKind = "u4k"
	iceLM4k deviceKind = "lm4k"
	ice5K   deviceKind = "5k"
	ice8K   deviceKind = "8k"
)

type tileKind string

const (
	tileCorner      tileKind = "corner"
	tileLogic       tileKind = "logic"
	tileRAMB        tileKind = "ramb"
	tileRAMT        tileKind = "ramt"
	tileIO          tileKind = "io"
	tileDSP0        tileKind = "dsp0"
	tileDSP1        tileKind = "dsp1"
	tileDSP2        tileKind = "dsp2"
	tileDSP3        tileKind = "dsp3"
	tileIPCon       tileKind = "ip_con"
	tileUnsupported tileKind = ""
)

func getFPGADevice(device string) *fpgaDevice {
	d := deviceKind(device)
	return knownFPGAs[d]
}

// [icepack]
var knownFPGAs = map[deviceKind]*fpgaDevice{
	ice384: {
		kind:       ice384,
		chipWidth:  6,
		chipHeight: 8,
		cramWidth:  182,
		cramHeight: 80,
		bramWidth:  0,
		bramHeight: 0,
		cols:       []int{18, 54, 54, 54, 54},
		tileKind: func(x, y int) tileKind {
			xEdge := x == 0 || x == 7
			yEdge := y == 0 || y == 9
			if xEdge && yEdge {
				return tileCorner
			}
			if xEdge || yEdge {
				return tileIO
			}
			return tileLogic
		},
	},

	ice1K: {
		kind:       ice1K,
		chipWidth:  12,
		chipHeight: 16,
		cramWidth:  332,
		cramHeight: 144,
		bramWidth:  64,
		bramHeight: 2 * 128,
		cols:       []int{18, 54, 54, 42, 54, 54, 54},
		tileKind: func(x, y int) tileKind {
			xEdge := x == 0 || x == 13
			yEdge := y == 0 || y == 17
			if xEdge && yEdge {
				return tileCorner
			}
			if xEdge || yEdge {
				return tileIO
			}
			if x == 3 || x == 10 {
				if y%2 == 1 {
					return tileRAMB
				}
				return tileRAMT
			}
			return tileLogic
		},
	},

	iceU4K: {
		kind:       iceU4K,
		chipWidth:  24,
		chipHeight: 20,
		cramWidth:  692,
		cramHeight: 176,
		bramWidth:  80,
		bramHeight: 2 * 128,
		cols:       []int{54, 54, 54, 54, 54, 54, 42, 54, 54, 54, 54, 54, 54},
		tileKind: func(x, y int) tileKind {
			xEdge := x == 0 || x == 25
			yEdge := y == 0 || y == 21
			if xEdge {
				if yEdge {
					return tileCorner
				}
				switch y {
				case 5, 13:
					return tileDSP0
				case 6, 14:
					return tileDSP1
				case 7, 15:
					return tileDSP2
				case 8, 16:
					return tileDSP3
				}
				return tileIPCon
			}
			if yEdge {
				return tileIO
			}

			if x == 6 || x == 19 {
				if y%2 == 1 {
					return tileRAMB
				}
				return tileRAMT
			}
			return tileLogic
		},
	},

	iceLM4k: {
		kind:       iceLM4k,
		chipWidth:  24,
		chipHeight: 20,
		cramWidth:  656,
		cramHeight: 176,
		bramWidth:  80,
		bramHeight: 2 * 128,
		cols:       []int{18, 54, 54, 54, 54, 54, 42, 54, 54, 54, 54, 54, 54},
		tileKind: func(x, y int) tileKind {
			xEdge := x == 0 || x == 25
			yEdge := y == 0 || y == 21
			if xEdge && yEdge {
				return tileCorner
			}
			if xEdge || yEdge {
				return tileIO
			}
			if x == 6 || x == 19 {
				if y%2 == 1 {
					return tileRAMB
				}
				return tileRAMT
			}
			return tileLogic
		},
	},

	ice5K: {
		kind:       ice5K,
		chipWidth:  24,
		chipHeight: 30,
		cramWidth:  692,
		cramHeight: 336,
		bramWidth:  160,
		bramHeight: 2 * 128,
		cols:       []int{54, 54, 54, 54, 54, 54, 42, 54, 54, 54, 54, 54, 54},
		tileKind: func(x, y int) tileKind {
			xEdge := x == 0 || x == 25
			yEdge := y == 0 || y == 31
			if xEdge {
				if yEdge {
					return tileCorner
				}
				switch y {
				case 5, 10, 15, 23:
					return tileDSP0
				case 6, 11, 16, 24:
					return tileDSP1
				case 7, 12, 17, 25:
					return tileDSP2
				case 8, 13, 18, 26:
					return tileDSP3
				}
				return tileIPCon
			}
			if xEdge || yEdge {
				return tileIO
			}

			if x == 6 || x == 19 {
				if y%2 == 1 {
					return tileRAMB
				}
				return tileRAMT
			}
			return tileLogic
		},
	},

	ice8K: {
		kind:       ice8K,
		chipWidth:  32,
		chipHeight: 32,
		cramWidth:  872,
		cramHeight: 272,
		bramWidth:  128,
		bramHeight: 2 * 128,
		cols:       []int{18, 54, 54, 54, 54, 54, 54, 54, 42, 54, 54, 54, 54, 54, 54, 54, 54},
		tileKind: func(x, y int) tileKind {
			xEdge := x == 0 || x == 33
			yEdge := y == 0 || y == 33
			if xEdge && yEdge {
				return tileCorner
			}
			if xEdge || yEdge {
				return tileIO
			}

			if x == 8 || x == 25 {
				if y%2 == 1 {
					return tileRAMB
				}
				return tileRAMT
			}
			return tileLogic
		},
	},
}

func (*fpgaDevice) tileWidth(t tileKind) int {
	switch t {
	case tileCorner:
		return 0
	case tileLogic:
		return 54
	case tileRAMB:
		return 42
	case tileRAMT:
		return 42
	case tileIO:
		return 18
	case tileDSP0, tileDSP1, tileDSP2, tileDSP3:
		return 54
	case tileIPCon:
		return 54
	}
	return 0
}
