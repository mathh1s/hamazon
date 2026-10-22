package worldmap

import (
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Dimension ids as cubiomes uses them.
type Dimension int

const (
	Overworld Dimension = 0
	Nether    Dimension = -1
	End       Dimension = 1
)

func ParseDimension(s string) (Dimension, bool) {
	switch s {
	case "overworld":
		return Overworld, true
	case "nether":
		return Nether, true
	case "end":
		return End, true
	}
	return 0, false
}

// TileSize is the edge of a tile in pixels; one pixel covers Scale blocks.
const TileSize = 128

// Scales are the zoom levels in blocks per pixel.
var Scales = []int{4, 16, 64}

var ErrBadTile = errors.New("unknown tile")

// Map renders biome tiles for one world seed, as the 1.21.4 generator
// produces them, and keeps them on disk since they never change.
type Map struct {
	seed     uint64
	cacheDir string
	colors   [256][3]uint8
	sem      chan struct{}

	spawnOnce sync.Once
	spawnX    int
	spawnZ    int
}

// ParseSeed accepts a numeric seed or, like the game, any text, which is
// turned into a number with Java's String.hashCode.
func ParseSeed(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty seed")
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return uint64(n), nil
	}
	var h int32
	for _, r := range s {
		h = 31*h + int32(r)
	}
	return uint64(int64(h)), nil
}

func New(seed uint64, cacheDir string) *Map {
	return &Map{seed: seed, cacheDir: cacheDir, colors: biomeColors(), sem: make(chan struct{}, 4)}
}

// Version identifies the seed without revealing it, for cache busting.
func (m *Map) Version() string {
	h := fnv.New32a()
	fmt.Fprintf(h, "%d", m.seed)
	return strconv.FormatUint(uint64(h.Sum32()), 36)
}

func (m *Map) Spawn() (int, int) {
	m.spawnOnce.Do(func() { m.spawnX, m.spawnZ = spawn(m.seed) })
	return m.spawnX, m.spawnZ
}

// Tile returns the PNG for tile tx, tz at the given scale. Tile 0, 0 has its
// top left corner at block 0, 0.
func (m *Map) Tile(dim Dimension, scale, tx, tz int) ([]byte, error) {
	if !validScale(scale) || abs(tx) > 1<<16 || abs(tz) > 1<<16 {
		return nil, ErrBadTile
	}
	path := filepath.Join(m.cacheDir, m.Version(), strconv.Itoa(int(dim)), strconv.Itoa(scale), fmt.Sprintf("%d_%d.png", tx, tz))
	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	}

	m.sem <- struct{}{}
	defer func() { <-m.sem }()

	ids, err := genBiomes(m.seed, dim, scale, tx*TileSize, tz*TileSize, TileSize, TileSize, sampleY(dim, scale))
	if err != nil {
		return nil, err
	}
	img := image.NewNRGBA(image.Rect(0, 0, TileSize, TileSize))
	for i, id := range ids {
		c := color.NRGBA{0, 0, 0, 255}
		if id >= 0 && id < 256 {
			rgb := m.colors[id]
			c = color.NRGBA{rgb[0], rgb[1], rgb[2], 255}
		}
		img.SetNRGBA(i%TileSize, i/TileSize, c)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
		_ = os.WriteFile(path, buf.Bytes(), 0o644)
	}
	return buf.Bytes(), nil
}

// sampleY picks the height the biomes are read at: sea level for the
// overworld and nether surface, in the units of the scale.
func sampleY(dim Dimension, scale int) int {
	if dim == End {
		return 0
	}
	return 64 / scale
}

func validScale(s int) bool {
	for _, v := range Scales {
		if v == s {
			return true
		}
	}
	return false
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

type Structure struct {
	Type string `json:"type"`
	X    int    `json:"x"`
	Z    int    `json:"z"`
}

// MaxStructureArea keeps a structure search cheap: about 8000 blocks across.
const MaxStructureArea = 8192

func (m *Map) Villages(x0, z0, x1, z1 int) ([]Structure, error) {
	if x1 < x0 || z1 < z0 || x1-x0 > MaxStructureArea || z1-z0 > MaxStructureArea {
		return nil, ErrBadTile
	}
	m.sem <- struct{}{}
	defer func() { <-m.sem }()
	return villages(m.seed, x0, z0, x1, z1), nil
}
