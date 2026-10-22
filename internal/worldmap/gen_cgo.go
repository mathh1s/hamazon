//go:build cgo

package worldmap

/*
#cgo CFLAGS: -O2 -w
#cgo LDFLAGS: -lm
#include <stdlib.h>
#include <string.h>
#include <math.h>
#include "cubiomes/generator.h"
#include "cubiomes/finders.h"
#include "cubiomes/util.h"

static int hz_gen(uint64_t seed, int dim, int scale, int x, int z, int w, int h, int y, int *out) {
	Generator g;
	setupGenerator(&g, MC_1_21_WD, 0);
	applySeed(&g, dim, seed);
	Range r = {scale, x, z, w, h, y, 1};
	int *cache = allocCache(&g, r);
	if (!cache) return -1;
	int err = genBiomes(&g, cache, r);
	if (!err) memcpy(out, cache, sizeof(int) * w * h);
	free(cache);
	return err;
}

static void hz_spawn(uint64_t seed, int *x, int *z) {
	Generator g;
	setupGenerator(&g, MC_1_21_WD, 0);
	applySeed(&g, DIM_OVERWORLD, seed);
	Pos p = getSpawn(&g);
	*x = p.x;
	*z = p.z;
}

// Finds villages with their biome in the block rectangle, up to max.
static int hz_villages(uint64_t seed, int x0, int z0, int x1, int z1, int *out, int max) {
	StructureConfig sc;
	if (!getStructureConfig(Village, MC_1_21_WD, &sc)) return 0;
	Generator g;
	setupGenerator(&g, MC_1_21_WD, 0);
	applySeed(&g, DIM_OVERWORLD, seed);
	int size = sc.regionSize * 16;
	int rx0 = (int)floor((double)x0 / size), rx1 = (int)floor((double)x1 / size);
	int rz0 = (int)floor((double)z0 / size), rz1 = (int)floor((double)z1 / size);
	int n = 0;
	for (int rz = rz0; rz <= rz1; rz++) {
		for (int rx = rx0; rx <= rx1; rx++) {
			Pos p;
			if (!getStructurePos(Village, MC_1_21_WD, seed, rx, rz, &p)) continue;
			if (p.x < x0 || p.x > x1 || p.z < z0 || p.z > z1) continue;
			if (!isViableStructurePos(Village, &g, p.x, p.z, 0)) continue;
			if (n >= max) return n;
			out[n*3] = p.x;
			out[n*3+1] = p.z;
			out[n*3+2] = getBiomeAt(&g, 4, p.x >> 2, 16, p.z >> 2);
			n++;
		}
	}
	return n;
}

static void hz_colors(unsigned char *out) {
	unsigned char c[256][3];
	initBiomeColors(c);
	memcpy(out, c, sizeof(c));
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

const Available = true

// genBiomes returns w*h biome ids starting at the scaled coordinates x, z.
func genBiomes(seed uint64, dim Dimension, scale, x, z, w, h, y int) ([]int32, error) {
	out := make([]int32, w*h)
	rc := C.hz_gen(C.uint64_t(seed), C.int(dim), C.int(scale), C.int(x), C.int(z), C.int(w), C.int(h), C.int(y),
		(*C.int)(unsafe.Pointer(&out[0])))
	if rc != 0 {
		return nil, errors.New("biome generation failed")
	}
	return out, nil
}

func spawn(seed uint64) (int, int) {
	var x, z C.int
	C.hz_spawn(C.uint64_t(seed), &x, &z)
	return int(x), int(z)
}

func villages(seed uint64, x0, z0, x1, z1 int) []Structure {
	const max = 256
	buf := make([]C.int, max*3)
	n := int(C.hz_villages(C.uint64_t(seed), C.int(x0), C.int(z0), C.int(x1), C.int(z1), &buf[0], max))
	out := make([]Structure, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, Structure{Type: villageType(int(buf[i*3+2])), X: int(buf[i*3]), Z: int(buf[i*3+1])})
	}
	return out
}

func biomeColors() [256][3]uint8 {
	var c [256][3]uint8
	C.hz_colors((*C.uchar)(unsafe.Pointer(&c[0][0])))
	return c
}

// villageType names the village style after the biome it stands in, which
// matches the map marker the game uses for it.
func villageType(biome int) string {
	switch biome {
	case C.desert:
		return "desert_village"
	case C.savanna, C.savanna_plateau:
		return "savanna_village"
	case C.snowy_plains, C.snowy_taiga:
		return "snowy_village"
	case C.taiga:
		return "taiga_village"
	}
	return "plains_village"
}
