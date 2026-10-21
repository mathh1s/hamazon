package assets

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"strings"
)

var (
	errMissing     = errors.New("missing resource")
	errUnsupported = errors.New("unsupported model")
)

// ==== item definitions (assets/<ns>/items/*.json) ====

type itemDefinition struct {
	Model json.RawMessage `json:"model"`
}

type itemModelNode struct {
	Type     string            `json:"type"`
	Model    json.RawMessage   `json:"model"`
	Tints    []tintSource      `json:"tints"`
	Property string            `json:"property"`
	Cases    []selectCase      `json:"cases"`
	Fallback json.RawMessage   `json:"fallback"`
	OnFalse  json.RawMessage   `json:"on_false"`
	Entries  []rangeEntry      `json:"entries"`
	Models   []json.RawMessage `json:"models"`
}

type selectCase struct {
	When  json.RawMessage `json:"when"`
	Model json.RawMessage `json:"model"`
}

type rangeEntry struct {
	Model json.RawMessage `json:"model"`
}

type tintSource struct {
	Type        string   `json:"type"`
	Value       *int64   `json:"value"`
	Default     *int64   `json:"default"`
	Temperature *float64 `json:"temperature"`
	Downfall    *float64 `json:"downfall"`
}

// modelRef is one model to draw plus the tint colors its faces may ask for.
type modelRef struct {
	id    string
	tints []tintSource
}

// resolveItem walks an item definition down to plain models, picking the
// branch the game would show in an inventory slot.
func resolveItem(raw json.RawMessage) ([]modelRef, error) {
	var n itemModelNode
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil, err
	}
	switch strings.TrimPrefix(n.Type, "minecraft:") {
	case "model":
		var id string
		if err := json.Unmarshal(n.Model, &id); err != nil {
			return nil, err
		}
		return []modelRef{{id: id, tints: n.Tints}}, nil
	case "select":
		if strings.TrimPrefix(n.Property, "minecraft:") == "display_context" {
			for _, c := range n.Cases {
				if whenContains(c.When, "gui") {
					return resolveItem(c.Model)
				}
			}
		}
		if len(n.Fallback) > 0 {
			return resolveItem(n.Fallback)
		}
		if len(n.Cases) > 0 {
			return resolveItem(n.Cases[0].Model)
		}
	case "condition":
		return resolveItem(n.OnFalse)
	case "range_dispatch":
		if len(n.Fallback) > 0 {
			return resolveItem(n.Fallback)
		}
		if len(n.Entries) > 0 {
			return resolveItem(n.Entries[0].Model)
		}
	case "composite":
		var out []modelRef
		for _, m := range n.Models {
			refs, err := resolveItem(m)
			if err != nil {
				return nil, err
			}
			out = append(out, refs...)
		}
		return out, nil
	}
	return nil, fmt.Errorf("%w: item model type %q", errUnsupported, n.Type)
}

func whenContains(raw json.RawMessage, want string) bool {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return one == want
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		for _, w := range many {
			if w == want {
				return true
			}
		}
	}
	return false
}

// ==== block and item models (assets/<ns>/models/*.json) ====

type rawModel struct {
	Parent   string                `json:"parent"`
	Textures map[string]textureRef `json:"textures"`
	Elements []element             `json:"elements"`
	Display  map[string]struct {
		Rotation    *[3]float64 `json:"rotation"`
		Translation *[3]float64 `json:"translation"`
		Scale       *[3]float64 `json:"scale"`
	} `json:"display"`
	GuiLight string `json:"gui_light"`
}

type element struct {
	From     [3]float64      `json:"from"`
	To       [3]float64      `json:"to"`
	Rotation *elemRotation   `json:"rotation"`
	Shade    *bool           `json:"shade"`
	Faces    map[string]face `json:"faces"`
}

type elemRotation struct {
	Origin  [3]float64 `json:"origin"`
	Axis    string     `json:"axis"`
	Angle   float64    `json:"angle"`
	Rescale bool       `json:"rescale"`
}

type face struct {
	UV        *[4]float64 `json:"uv"`
	Texture   string      `json:"texture"`
	TintIndex *int        `json:"tintindex"`
	Rotation  int         `json:"rotation"`
}

type transform struct {
	rotation, translation, scale [3]float64
}

// textureRef is either a plain texture id or, since 1.21.x, an object
// carrying the sprite id plus render hints.
type textureRef string

func (t *textureRef) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*t = textureRef(s)
		return nil
	}
	var obj struct {
		Sprite string `json:"sprite"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	*t = textureRef(obj.Sprite)
	return nil
}

// model is a fully merged model: textures from the whole parent chain,
// elements and gui transform from the nearest model that defines them.
type model struct {
	generated bool
	textures  map[string]string
	elements  []element
	gui       transform
	sideLight bool
}

func (p *pack) loadModel(id string) (*model, error) {
	m := &model{textures: map[string]string{}, sideLight: true}
	haveElements, haveGui, haveLight := false, false, false

	for depth := 0; id != ""; depth++ {
		if depth > 32 {
			return nil, fmt.Errorf("model parent chain too deep at %s", id)
		}
		switch strings.TrimPrefix(id, "minecraft:") {
		case "builtin/generated":
			m.generated = true
			return m, nil
		case "builtin/entity":
			return nil, fmt.Errorf("%w: builtin/entity", errUnsupported)
		}

		var raw rawModel
		if err := p.readJSON(resourcePath(id, "models", ".json"), &raw); err != nil {
			return nil, err
		}
		for k, v := range raw.Textures {
			if _, set := m.textures[k]; !set {
				m.textures[k] = string(v)
			}
		}
		if !haveElements && raw.Elements != nil {
			m.elements, haveElements = raw.Elements, true
		}
		if gui, ok := raw.Display["gui"]; ok && !haveGui {
			m.gui = transform{scale: [3]float64{1, 1, 1}}
			if gui.Rotation != nil {
				m.gui.rotation = *gui.Rotation
			}
			if gui.Translation != nil {
				m.gui.translation = *gui.Translation
			}
			if gui.Scale != nil {
				m.gui.scale = *gui.Scale
			}
			haveGui = true
		}
		if raw.GuiLight != "" && !haveLight {
			m.sideLight, haveLight = raw.GuiLight != "front", true
		}
		id = raw.Parent
	}
	if !haveGui {
		m.gui = transform{scale: [3]float64{1, 1, 1}}
	}
	if len(m.elements) == 0 {
		return nil, fmt.Errorf("%w: model has no elements", errUnsupported)
	}
	return m, nil
}

// resolveTexture follows "#key" references through the merged texture map.
func (m *model) resolveTexture(ref string) (string, bool) {
	for i := 0; i < 16; i++ {
		key, isRef := strings.CutPrefix(ref, "#")
		if !isRef {
			// Some models name a texture variable without the leading #.
			if _, ok := m.textures[ref]; !ok || strings.ContainsAny(ref, ":/") {
				return ref, ref != ""
			}
		}
		next, ok := m.textures[key]
		if !ok {
			return "", false
		}
		ref = next
	}
	return "", false
}

// ==== tints ====

func (p *pack) tintColor(src tintSource) color.NRGBA {
	switch strings.TrimPrefix(src.Type, "minecraft:") {
	case "grass":
		if src.Temperature != nil && src.Downfall != nil {
			if c, ok := p.colormap("minecraft:colormap/grass", *src.Temperature, *src.Downfall); ok {
				return c
			}
		}
	}
	if src.Value != nil {
		return argb(*src.Value)
	}
	if src.Default != nil {
		return argb(*src.Default)
	}
	return color.NRGBA{255, 255, 255, 255}
}

// colormap samples a biome colormap the way the game does for a given
// temperature and downfall.
func (p *pack) colormap(id string, temperature, downfall float64) (color.NRGBA, bool) {
	img, err := p.texture(id)
	if err != nil {
		return color.NRGBA{}, false
	}
	t := clamp01(temperature)
	d := clamp01(downfall) * t
	x := int((1 - t) * 255)
	y := int((1 - d) * 255)
	b := img.Bounds()
	if x >= b.Dx() || y >= b.Dy() {
		return color.NRGBA{}, false
	}
	return img.NRGBAAt(x, y), true
}

func argb(v int64) color.NRGBA {
	u := uint32(v)
	return color.NRGBA{R: uint8(u >> 16), G: uint8(u >> 8), B: uint8(u), A: 255}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
