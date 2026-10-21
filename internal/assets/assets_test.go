package assets

import (
	"encoding/json"
	"testing"
)

func TestResolveItemPicksInventoryBranch(t *testing.T) {
	raw := json.RawMessage(`{"model":{"type":"minecraft:select","property":"minecraft:display_context",
		"cases":[{"when":["gui","ground"],"model":{"type":"minecraft:model","model":"minecraft:item/trident"}}],
		"fallback":{"type":"minecraft:special","base":"minecraft:item/trident_in_hand","model":{"type":"minecraft:trident"}}}}`)
	var def itemDefinition
	if err := json.Unmarshal(raw, &def); err != nil {
		t.Fatal(err)
	}
	refs, err := resolveItem(def.Model)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].id != "minecraft:item/trident" {
		t.Fatalf("got %+v", refs)
	}
}

func TestMergeRectsStacksRuns(t *testing.T) {
	got := mergeRects([]rect{{0, 0, 2, 1}, {0, 1, 2, 2}, {3, 0, 4, 1}})
	if len(got) != 2 || got[0] != (rect{0, 0, 2, 2}) {
		t.Fatalf("got %+v", got)
	}
}

func TestBuildTrueTypeChecksum(t *testing.T) {
	font, err := buildTrueType(map[rune]glyph{'A': {r: 'A', advance: 6 * fontUnit, rects: []rect{{0, 0, 5 * fontUnit, 7 * fontUnit}}}}, "Test")
	if err != nil {
		t.Fatal(err)
	}
	if checksum(append(font, make([]byte, (4-len(font)%4)%4)...)) != 0xB1B0AFBA {
		t.Fatal("whole font checksum must equal the magic constant")
	}
}
