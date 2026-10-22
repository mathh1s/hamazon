//go:build !cgo

package worldmap

import "errors"

// Without cgo there is no biome generator, the map is simply turned off.
const Available = false

var errNoCgo = errors.New("this build has no biome generator, build with CGO_ENABLED=1")

func genBiomes(seed uint64, dim Dimension, scale, x, z, w, h, y int) ([]int32, error) {
	return nil, errNoCgo
}

func spawn(seed uint64) (int, int) { return 0, 0 }

func biomeColors() [256][3]uint8 { return [256][3]uint8{} }

func villages(seed uint64, x0, z0, x1, z1 int) []Structure { return nil }
