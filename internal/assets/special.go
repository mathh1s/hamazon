package assets

import (
	"encoding/json"
	"fmt"
	"image"
	"strings"
)

// Items like chests, banners and heads are drawn by dedicated entity
// renderers in the game, not by a model file. These builders recreate the
// shape of each one from cuboids on its entity texture, so the regular
// element renderer can draw them like any block.

type specialSpec struct {
	Type    string `json:"type"`
	Texture string `json:"texture"`
	Kind    string `json:"kind"`
	Color   string `json:"color"`
	Base    string `json:"-"`
}

func parseSpecial(n itemModelNode) (*specialSpec, error) {
	var spec specialSpec
	if err := json.Unmarshal(n.Model, &spec); err != nil {
		return nil, err
	}
	spec.Type = strings.TrimPrefix(spec.Type, "minecraft:")
	spec.Base = n.Base
	return &spec, nil
}

// box describes one cuboid in block pixels (0 to 16, y up) and where its
// faces sit on the texture, using the game's box unwrap layout.
type box struct {
	from, to   vec3
	u, v       float64
	w, h, d    float64
	tex        string
	tint       int
	swapTop    bool
	mirrorSide bool
}

// front is always the north face; renderSpecial turns the model so it ends
// up where the game shows it.
func (b box) element(tw, th float64) element {
	uv := func(x1, y1, x2, y2 float64) *[4]float64 {
		return &[4]float64{x1 * 16 / tw, y1 * 16 / th, x2 * 16 / tw, y2 * 16 / th}
	}
	u, v, w, h, d := b.u, b.v, b.w, b.h, b.d
	top := uv(u+d+w, v+d, u+d, v)
	bottom := uv(u+d+w+w, v, u+d+w, v+d)
	if b.swapTop {
		top, bottom = uv(u+d+w+w, v+d, u+d+w, v), uv(u+d+w, v, u+d, v+d)
	}
	east, west := uv(u, v+d, u+d, v+d+h), uv(u+d+w, v+d, u+d+w+d, v+d+h)
	if b.mirrorSide {
		east, west = uv(u+d, v+d, u, v+d+h), uv(u+d+w+d, v+d, u+d+w, v+d+h)
	}
	faces := map[string]face{
		"north": {UV: uv(u+d, v+d, u+d+w, v+d+h)},
		"south": {UV: uv(u+d+w+d, v+d, u+d+w+d+w, v+d+h)},
		"east":  {UV: east},
		"west":  {UV: west},
		"up":    {UV: top},
		"down":  {UV: bottom},
	}
	for k, f := range faces {
		f.Texture = "#" + b.tex
		if b.tint > 0 {
			idx := b.tint - 1
			f.TintIndex = &idx
		}
		faces[k] = f
	}
	return element{From: [3]float64(b.from), To: [3]float64(b.to), Faces: faces}
}

func inflate(b box, by float64) box {
	b.from = vec3{b.from[0] - by, b.from[1] - by, b.from[2] - by}
	b.to = vec3{b.to[0] + by, b.to[1] + by, b.to[2] + by}
	return b
}

type specialModel struct {
	textures map[string]string
	sizes    map[string][2]float64
	boxes    []box
	tints    []tintSource
}

var dyeColors = map[string]int64{
	"white": 0xF9FFFE, "orange": 0xF9801D, "magenta": 0xC74EBD, "light_blue": 0x3AB3DA,
	"yellow": 0xFED83D, "lime": 0x80C71F, "pink": 0xF38BAA, "gray": 0x474F52,
	"light_gray": 0x9D9D97, "cyan": 0x169C9C, "purple": 0x8932B8, "blue": 0x3C44AA,
	"brown": 0x835432, "green": 0x5E7C16, "red": 0xB02E26, "black": 0x1D1D21,
}

func textureID(raw, folder string) string {
	name := strings.TrimPrefix(raw, "minecraft:")
	name = strings.TrimPrefix(name, "textures/")
	name = strings.TrimSuffix(name, ".png")
	if strings.Contains(name, "/") {
		return "minecraft:" + name
	}
	return "minecraft:" + folder + "/" + name
}

func buildSpecial(spec *specialSpec) (*specialModel, error) {
	m := &specialModel{textures: map[string]string{}, sizes: map[string][2]float64{}}
	tex := func(key, id string, w, h float64) {
		m.textures[key] = id
		m.sizes[key] = [2]float64{w, h}
	}

	switch spec.Type {
	case "chest":
		tex("t", textureID(spec.Texture, "entity/chest"), 64, 64)
		m.boxes = []box{
			{from: vec3{1, 0, 1}, to: vec3{15, 10, 15}, u: 0, v: 19, w: 14, h: 10, d: 14, tex: "t", swapTop: true},
			{from: vec3{1, 9, 1}, to: vec3{15, 14, 15}, u: 0, v: 0, w: 14, h: 5, d: 14, tex: "t", swapTop: true},
			{from: vec3{7, 7, 0}, to: vec3{9, 11, 1}, u: 0, v: 0, w: 2, h: 4, d: 1, tex: "t"},
		}

	case "shulker_box":
		tex("t", textureID(spec.Texture, "entity/shulker"), 64, 64)
		m.boxes = []box{
			{from: vec3{0, 0, 0}, to: vec3{16, 8, 16}, u: 0, v: 28, w: 16, h: 8, d: 16, tex: "t"},
			{from: vec3{0, 4, 0}, to: vec3{16, 16, 16}, u: 0, v: 0, w: 16, h: 12, d: 16, tex: "t"},
		}

	case "banner":
		color, ok := dyeColors[spec.Color]
		if !ok {
			return nil, fmt.Errorf("%w: banner color %q", errUnsupported, spec.Color)
		}
		tex("pole", "minecraft:entity/banner/banner_base", 64, 64)
		tex("flag", "minecraft:entity/banner/base", 64, 64)
		m.tints = []tintSource{{Value: &color}}
		m.boxes = []box{
			{from: vec3{7, -14, 7}, to: vec3{9, 28, 9}, u: 44, v: 0, w: 2, h: 42, d: 2, tex: "pole"},
			{from: vec3{-2, 28, 7}, to: vec3{18, 30, 9}, u: 0, v: 42, w: 20, h: 2, d: 2, tex: "pole"},
			{from: vec3{-2, -12, 6}, to: vec3{18, 28, 7}, u: 0, v: 0, w: 20, h: 40, d: 1, tex: "flag", tint: 1},
		}

	case "head":
		return buildHead(m, spec.Kind)
	case "player_head":
		return buildHead(m, "player")

	case "shield":
		tex("t", "minecraft:entity/shield/shield_base_nopattern", 64, 64)
		m.boxes = []box{
			{from: vec3{2, -3, 7}, to: vec3{14, 19, 8}, u: 0, v: 0, w: 12, h: 22, d: 1, tex: "t"},
			{from: vec3{7, 5, 8}, to: vec3{9, 11, 14}, u: 26, v: 0, w: 2, h: 6, d: 6, tex: "t"},
		}

	case "conduit":
		tex("t", "minecraft:entity/conduit/base", 32, 16)
		m.boxes = []box{{from: vec3{5, 5, 5}, to: vec3{11, 11, 11}, u: 0, v: 0, w: 6, h: 6, d: 6, tex: "t"}}

	case "decorated_pot":
		tex("base", "minecraft:entity/decorated_pot/decorated_pot_base", 32, 32)
		tex("side", "minecraft:entity/decorated_pot/decorated_pot_side", 16, 16)
		m.boxes = []box{{from: vec3{4, 16, 4}, to: vec3{12, 20, 12}, u: 0, v: 0, w: 8, h: 4, d: 8, tex: "base"}}
		return m, nil

	case "copper_golem_statue":
		tex("t", textureID(spec.Texture, "entity/copper_golem"), 64, 64)
		m.boxes = []box{
			{from: vec3{4, 0, 6}, to: vec3{8, 5, 10}, u: 0, v: 27, w: 4, h: 5, d: 4, tex: "t"},
			{from: vec3{8, 0, 6}, to: vec3{12, 5, 10}, u: 16, v: 27, w: 4, h: 5, d: 4, tex: "t"},
			{from: vec3{4, 5, 5}, to: vec3{12, 11, 11}, u: 0, v: 15, w: 8, h: 6, d: 6, tex: "t"},
			{from: vec3{12, 1, 6}, to: vec3{15, 11, 10}, u: 36, v: 16, w: 3, h: 10, d: 4, tex: "t"},
			{from: vec3{1, 1, 6}, to: vec3{4, 11, 10}, u: 50, v: 16, w: 3, h: 10, d: 4, tex: "t"},
			{from: vec3{4, 11, 3}, to: vec3{12, 16, 13}, u: 0, v: 0, w: 8, h: 5, d: 10, tex: "t"},
			{from: vec3{7, 11, 2}, to: vec3{9, 14, 4}, u: 56, v: 0, w: 2, h: 3, d: 2, tex: "t"},
			{from: vec3{7, 16, 7}, to: vec3{9, 20, 9}, u: 37, v: 8, w: 2, h: 4, d: 2, tex: "t"},
			{from: vec3{6, 20, 6}, to: vec3{10, 24, 10}, u: 37, v: 0, w: 4, h: 4, d: 4, tex: "t"},
		}

	default:
		return nil, fmt.Errorf("%w: special model %q", errUnsupported, spec.Type)
	}
	return m, nil
}

func buildHead(m *specialModel, kind string) (*specialModel, error) {
	type skin struct {
		texture string
		w, h    float64
		hat     bool
	}
	skins := map[string]skin{
		"skeleton":        {"minecraft:entity/skeleton/skeleton", 64, 32, false},
		"wither_skeleton": {"minecraft:entity/skeleton/wither_skeleton", 64, 32, false},
		"zombie":          {"minecraft:entity/zombie/zombie", 64, 64, true},
		"creeper":         {"minecraft:entity/creeper/creeper", 64, 32, false},
		"player":          {"minecraft:entity/player/wide/steve", 64, 64, true},
	}
	switch kind {
	case "piglin":
		m.textures["t"] = "minecraft:entity/piglin/piglin"
		m.sizes["t"] = [2]float64{64, 64}
		m.boxes = []box{
			{from: vec3{3, 0, 4}, to: vec3{13, 8, 12}, u: 0, v: 0, w: 10, h: 8, d: 8, tex: "t"},
			{from: vec3{6, 0, 3}, to: vec3{10, 4, 4}, u: 31, v: 1, w: 4, h: 4, d: 1, tex: "t"},
			{from: vec3{2, 2, 6}, to: vec3{3, 7, 10}, u: 39, v: 6, w: 1, h: 5, d: 4, tex: "t"},
			{from: vec3{13, 2, 6}, to: vec3{14, 7, 10}, u: 51, v: 6, w: 1, h: 5, d: 4, tex: "t"},
		}
		return m, nil
	case "dragon":
		m.textures["t"] = "minecraft:entity/enderdragon/dragon"
		m.sizes["t"] = [2]float64{256, 256}
		m.boxes = []box{
			{from: vec3{0, 0, 4}, to: vec3{16, 16, 20}, u: 112, v: 30, w: 16, h: 16, d: 16, tex: "t"},
			{from: vec3{2, 4, -12}, to: vec3{14, 9, 4}, u: 176, v: 44, w: 12, h: 5, d: 16, tex: "t"},
			{from: vec3{2, 0, -12}, to: vec3{14, 4, 4}, u: 176, v: 65, w: 12, h: 4, d: 16, tex: "t"},
		}
		return m, nil
	}
	s, ok := skins[kind]
	if !ok {
		return nil, fmt.Errorf("%w: head %q", errUnsupported, kind)
	}
	m.textures["t"] = s.texture
	m.sizes["t"] = [2]float64{s.w, s.h}
	head := box{from: vec3{4, 0, 4}, to: vec3{12, 8, 12}, u: 0, v: 0, w: 8, h: 8, d: 8, tex: "t"}
	m.boxes = []box{head}
	if s.hat {
		hat := head
		hat.u = 32
		m.boxes = append(m.boxes, inflate(hat, 0.5))
	}
	return m, nil
}

// specialTransforms holds the inventory pose per renderer. The game's own
// display values assume its entity space, which is turned 180 degrees
// against the block space used here.
var specialTransforms = map[string]transform{
	"chest":               {rotation: [3]float64{30, 225, 0}, scale: [3]float64{0.625, 0.625, 0.625}},
	"shulker_box":         {rotation: [3]float64{30, 225, 0}, scale: [3]float64{0.625, 0.625, 0.625}},
	"banner":              {rotation: [3]float64{30, 200, 0}, translation: [3]float64{0, -3.25, 0}, scale: [3]float64{0.5325, 0.5325, 0.5325}},
	"head":                {rotation: [3]float64{30, 225, 0}, translation: [3]float64{0, 3, 0}, scale: [3]float64{1, 1, 1}},
	"player_head":         {rotation: [3]float64{30, 225, 0}, translation: [3]float64{0, 3, 0}, scale: [3]float64{1, 1, 1}},
	"shield":              {rotation: [3]float64{15, 155, 5}, translation: [3]float64{-2, 3, 0}, scale: [3]float64{0.65, 0.65, 0.65}},
	"conduit":             {rotation: [3]float64{30, 225, 0}, scale: [3]float64{1, 1, 1}},
	"decorated_pot":       {rotation: [3]float64{30, 225, 0}, scale: [3]float64{0.6, 0.6, 0.6}},
	"copper_golem_statue": {rotation: [3]float64{30, 225, 0}, translation: [3]float64{0, -2, 0}, scale: [3]float64{0.55, 0.55, 0.55}},
}

var specialFrontLight = map[string]bool{"banner": true, "shield": true, "decorated_pot": true}

func (p *pack) renderSpecial(spec *specialSpec) (*image.NRGBA, error) {
	sm, err := buildSpecial(spec)
	if err != nil {
		return nil, err
	}
	m := &model{textures: sm.textures, gui: specialTransforms[spec.Type], sideLight: !specialFrontLight[spec.Type]}
	if spec.Type == "head" && spec.Kind == "dragon" {
		m.gui = transform{rotation: [3]float64{30, 225, 0}, translation: [3]float64{2, 2, 0}, scale: [3]float64{0.6, 0.6, 0.6}}
	}
	for _, b := range sm.boxes {
		size := sm.sizes[b.tex]
		m.elements = append(m.elements, b.element(size[0], size[1]))
	}
	if spec.Type == "decorated_pot" {
		m.elements = append(m.elements, potBody())
	}
	return p.renderElements(m, sm.tints)
}

// potBody uses the plain side texture on all four sides and the top and
// bottom squares of the base texture.
func potBody() element {
	side := face{UV: &[4]float64{1, 0, 15, 16}, Texture: "#side"}
	return element{
		From: [3]float64{1, 0, 1}, To: [3]float64{15, 16, 15},
		Faces: map[string]face{
			"north": side, "south": side, "east": side, "west": side,
			"up":   {UV: &[4]float64{0, 6.5, 7, 13.5}, Texture: "#base"},
			"down": {UV: &[4]float64{7, 6.5, 14, 13.5}, Texture: "#base"},
		},
	}
}
