package assets

import (
	"fmt"
	"image"
	"image/draw"
)

// guiSprites maps output names to textures copied as they are.
var guiSprites = map[string]string{
	"button":               "minecraft:gui/sprites/widget/button",
	"button_highlighted":   "minecraft:gui/sprites/widget/button_highlighted",
	"button_disabled":      "minecraft:gui/sprites/widget/button_disabled",
	"text_field":           "minecraft:gui/sprites/widget/text_field",
	"text_field_highlight": "minecraft:gui/sprites/widget/text_field_highlighted",
	"page_forward":         "minecraft:gui/sprites/widget/page_forward",
	"page_backward":        "minecraft:gui/sprites/widget/page_backward",
	"slot":                 "minecraft:gui/sprites/container/slot",
	"tab":                  "minecraft:gui/sprites/container/creative_inventory/tab_top_unselected_2",
	"tab_selected":         "minecraft:gui/sprites/container/creative_inventory/tab_top_selected_2",
	"tab_bottom":           "minecraft:gui/sprites/container/creative_inventory/tab_bottom_unselected_2",
	"tab_bottom_selected":  "minecraft:gui/sprites/container/creative_inventory/tab_bottom_selected_2",
	"scroller":             "minecraft:gui/sprites/container/creative_inventory/scroller",
	"tooltip_background":   "minecraft:gui/sprites/tooltip/background",
	"tooltip_frame":        "minecraft:gui/sprites/tooltip/frame",
	"dirt":                 "minecraft:block/dirt",
	"stone":                "minecraft:block/stone",
	"toast":                "minecraft:gui/sprites/toast/advancement",
	"hotbar":               "minecraft:gui/sprites/hud/hotbar",
	"hotbar_selection":     "minecraft:gui/sprites/hud/hotbar_selection",
}

// guiCrops cut a region out of a larger GUI sheet.
var guiCrops = map[string]struct {
	texture string
	rect    image.Rectangle
}{
	"book": {"minecraft:gui/book", image.Rect(20, 1, 166, 181)},
}

func (p *pack) guiImages() (map[string]*image.NRGBA, error) {
	out := map[string]*image.NRGBA{}
	for name, id := range guiSprites {
		img, err := p.texture(id)
		if err != nil {
			return nil, fmt.Errorf("gui sprite %s: %w", name, err)
		}
		out[name] = img
	}
	for name, crop := range guiCrops {
		img, err := p.texture(crop.texture)
		if err != nil {
			return nil, fmt.Errorf("gui crop %s: %w", name, err)
		}
		dst := image.NewNRGBA(image.Rect(0, 0, crop.rect.Dx(), crop.rect.Dy()))
		draw.Draw(dst, dst.Bounds(), img, crop.rect.Min, draw.Src)
		out[name] = dst
	}

	panel, err := p.panelSprite()
	if err != nil {
		return nil, err
	}
	out["panel"] = panel
	return out, nil
}

// panelSprite builds a 12x12 nine-slice source from the chest screen, so
// panels of any size get the exact rounded container border.
func (p *pack) panelSprite() (*image.NRGBA, error) {
	chest, err := p.texture("minecraft:gui/container/generic_54")
	if err != nil {
		return nil, err
	}
	const w, h = 176, 222
	src := func(i, size int) int {
		switch {
		case i < 4:
			return i
		case i < 8:
			return 8 + i - 4
		default:
			return size - 4 + i - 8
		}
	}
	out := image.NewNRGBA(image.Rect(0, 0, 12, 12))
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			out.SetNRGBA(x, y, chest.NRGBAAt(src(x, w), src(y, h)))
		}
	}
	return out, nil
}
