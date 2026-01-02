package gice

type fpgaParams struct {
	width  int
	height int
	cols   []int

	tileType func(x, y int) tileType
}

type fpgaDevice string

const (
	ice384  fpgaDevice = "384"
	ice1K   fpgaDevice = "1k"
	iceU4K  fpgaDevice = "u4k"
	iceLM4k fpgaDevice = "lm4k"
	ice5K   fpgaDevice = "5k"
	ice8K   fpgaDevice = "8k"
)

type tileType int

const (
	tileTypeCorner = iota
	tileTypeLogic
	tileTypeRAMB
	tileTypeRAMT
	tileTypeIO
	tileTypeDSP0
	tileTypeDSP1
	tileTypeDSP2
	tileTypeDSP3
	tileTypeIPCon
	tileTypeUnsupported
)

// [icepack]
var knownFPGAs = map[fpgaDevice]fpgaParams{
	ice384: {
		width:  6,
		height: 8,
		cols:   []int{18, 54, 54, 54, 54},
		tileType: func(x, y int) tileType {
			xEdge := x == 0 || x == 7
			yEdge := y == 0 || y == 9
			if xEdge && yEdge {
				return tileTypeCorner
			}
			if xEdge || yEdge {
				return tileTypeIO
			}
			return tileTypeLogic
		},
	},

	ice1K: {
		width:  12,
		height: 16,
		cols:   []int{18, 54, 54, 42, 54, 54, 54},
		tileType: func(x, y int) tileType {
			xEdge := x == 0 || x == 13
			yEdge := y == 0 || y == 17
			if xEdge && yEdge {
				return tileTypeCorner
			}
			if xEdge || yEdge {
				return tileTypeIO
			}
			if x == 3 || x == 10 {
				if y%2 == 1 {
					return tileTypeRAMB
				}
				return tileTypeRAMT
			}
			return tileTypeLogic
		},
	},

	iceU4K: {
		width:  24,
		height: 20,
		cols:   []int{54, 54, 54, 54, 54, 54, 42, 54, 54, 54, 54, 54, 54},
		tileType: func(x, y int) tileType {
			xEdge := x == 0 || x == 25
			yEdge := y == 0 || y == 21
			if xEdge {
				if yEdge {
					return tileTypeCorner
				}
				switch y {
				case 5, 13:
					return tileTypeDSP0
				case 6, 14:
					return tileTypeDSP1
				case 7, 15:
					return tileTypeDSP2
				case 8, 16:
					return tileTypeDSP3
				}
				return tileTypeIPCon
			}
			if yEdge {
				return tileTypeIO
			}

			if x == 6 || x == 19 {
				if y%2 == 1 {
					return tileTypeRAMB
				}
				return tileTypeRAMT
			}
			return tileTypeLogic
		},
	},

	iceLM4k: {
		width:  24,
		height: 20,
		cols:   []int{18, 54, 54, 54, 54, 54, 42, 54, 54, 54, 54, 54, 54},
		tileType: func(x, y int) tileType {
			xEdge := x == 0 || x == 25
			yEdge := y == 0 || y == 21
			if xEdge && yEdge {
				return tileTypeCorner
			}
			if xEdge || yEdge {
				return tileTypeIO
			}
			if x == 6 || x == 19 {
				if y%2 == 1 {
					return tileTypeRAMB
				}
				return tileTypeRAMT
			}
			return tileTypeLogic
		},
	},

	ice5K: {
		width:  24,
		height: 30,
		cols:   []int{54, 54, 54, 54, 54, 54, 42, 54, 54, 54, 54, 54, 54},
		tileType: func(x, y int) tileType {
			xEdge := x == 0 || x == 25
			yEdge := y == 0 || y == 31
			if xEdge {
				if yEdge {
					return tileTypeCorner
				}
				switch y {
				case 5, 10, 15, 23:
					return tileTypeDSP0
				case 6, 11, 16, 24:
					return tileTypeDSP1
				case 7, 12, 17, 25:
					return tileTypeDSP2
				case 8, 13, 18, 26:
					return tileTypeDSP3
				}
				return tileTypeIPCon
			}
			if xEdge || yEdge {
				return tileTypeIO
			}

			if x == 6 || x == 19 {
				if y%2 == 1 {
					return tileTypeRAMB
				}
				return tileTypeRAMT
			}
			return tileTypeLogic
		},
	},

	ice8K: {
		width:  32,
		height: 32,
		cols:   []int{18, 54, 54, 54, 54, 54, 54, 54, 42, 54, 54, 54, 54, 54, 54, 54, 54},
		tileType: func(x, y int) tileType {
			xEdge := x == 0 || x == 33
			yEdge := y == 0 || y == 33
			if xEdge && yEdge {
				return tileTypeCorner
			}
			if xEdge || yEdge {
				return tileTypeIO
			}

			if x == 8 || x == 25 {
				if y%2 == 1 {
					return tileTypeRAMB
				}
				return tileTypeRAMT
			}
			return tileTypeLogic
		},
	},
}

func (f *fpgaParams) tileWidth(t tileType) int {
	switch t {
	case tileTypeCorner:
		return 0
	case tileTypeLogic:
		return 54
	case tileTypeRAMB:
		return 42
	case tileTypeRAMT:
		return 42
	case tileTypeIO:
		return 18
	case tileTypeDSP0, tileTypeDSP1, tileTypeDSP2, tileTypeDSP3:
		return 54
	case tileTypeIPCon:
		return 54
	}
	return 0
}
