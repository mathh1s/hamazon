package worldmap

import (
	"bytes"
	"image/png"
	"testing"
)

func TestParseSeed(t *testing.T) {
	if s, _ := ParseSeed("6715050767882635398"); s != 6715050767882635398 {
		t.Fatalf("numeric seed parsed wrong: %d", s)
	}
	if s, _ := ParseSeed("-42"); int64(s) != -42 {
		t.Fatalf("negative seed parsed wrong: %d", int64(s))
	}
	// "Hello" hashes to 69609650 in Java.
	if s, _ := ParseSeed("Hello"); int64(s) != 69609650 {
		t.Fatalf("text seed must use String.hashCode, got %d", int64(s))
	}
}

func TestTileRenders(t *testing.T) {
	if !Available {
		t.Skip("built without cgo")
	}
	m := New(6715050767882635398, t.TempDir())
	data, err := m.Tile(Overworld, 16, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds().Dx() != TileSize {
		t.Fatalf("bad tile: %v", err)
	}
	cached, _ := m.Tile(Overworld, 16, 0, 0)
	if !bytes.Equal(data, cached) {
		t.Fatal("second request should come from the cache")
	}
	if _, err := m.Tile(Overworld, 5, 0, 0); err != ErrBadTile {
		t.Fatal("unsupported scales must be rejected")
	}
	if _, err := m.Tile(Nether, 4, -1, 2); err != nil {
		t.Fatal(err)
	}
	x, z := m.Spawn()
	if abs(x) > 2000 || abs(z) > 2000 {
		t.Fatalf("spawn looks wrong: %d %d", x, z)
	}
}

func TestVillagesNearSpawn(t *testing.T) {
	if !Available {
		t.Skip("built without cgo")
	}
	m := New(6715050767882635398, t.TempDir())
	list, err := m.Villages(-3000, -3000, 3000, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("expected some villages within 3000 blocks of spawn")
	}
	t.Logf("%d villages, first %+v", len(list), list[0])
	if _, err := m.Villages(0, 0, 100000, 10); err != ErrBadTile {
		t.Fatal("huge searches must be refused")
	}
}
