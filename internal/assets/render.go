package assets

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
	"strconv"
)

const (
	iconSize    = 128
	supersample = 2
)

type iconKind string

const (
	kindFlat  iconKind = "flat"
	kindBlock iconKind = "block"
)

func (p *pack) renderItem(refs []modelRef) (*image.NRGBA, iconKind, error) {
	out := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
	kind := kindFlat
	for _, ref := range refs {
		m, err := p.loadModel(ref.id)
		if err != nil {
			return nil, "", err
		}
		var layer *image.NRGBA
		if m.generated {
			layer, err = p.renderGenerated(m, ref.tints)
		} else {
			kind = kindBlock
			layer, err = p.renderElements(m, ref.tints)
		}
		if err != nil {
			return nil, "", err
		}
		over(out, layer)
	}
	if isEmpty(out) {
		return nil, "", fmt.Errorf("%w: renders empty", errUnsupported)
	}
	return out, kind, nil
}

// ==== flat items ====

func (p *pack) renderGenerated(m *model, tints []tintSource) (*image.NRGBA, error) {
	out := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
	found := false
	for i := 0; i < 8; i++ {
		ref, ok := m.resolveTexture("#layer" + strconv.Itoa(i))
		if !ok {
			continue
		}
		tex, err := p.texture(ref)
		if err != nil {
			continue
		}
		found = true
		tint := color.NRGBA{255, 255, 255, 255}
		if i < len(tints) {
			tint = p.tintColor(tints[i])
		}
		w := tex.Bounds().Dx()
		for y := 0; y < iconSize; y++ {
			for x := 0; x < iconSize; x++ {
				c := tex.NRGBAAt(x*w/iconSize, y*tex.Bounds().Dy()/iconSize)
				if c.A == 0 {
					continue
				}
				blend(out, x, y, multiply(c, tint, 1))
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("%s: no layer textures", errMissing)
	}
	return out, nil
}

// ==== block items ====

type vec3 [3]float64

type quad struct {
	p00, p10, p01 vec3 // screen x, screen y, depth for the face corners
	tex           *image.NRGBA
	uv            [4]float64
	rotation      int
	tint          color.NRGBA
	shade         float64
	translucent   bool
}

// renderElements draws cuboid models the way the inventory shows them,
// applying the model's gui display transform and a fixed side light.
func (p *pack) renderElements(m *model, tints []tintSource) (*image.NRGBA, error) {
	size := iconSize * supersample
	var quads []quad

	for _, el := range m.elements {
		for dir, f := range el.Faces {
			ref, ok := m.resolveTexture(f.Texture)
			if !ok {
				continue
			}
			tex, err := p.texture(ref)
			if err != nil {
				continue
			}
			corners, normal, ok := faceCorners(el, dir)
			if !ok {
				continue
			}
			for i := range corners {
				corners[i] = applyGui(m.gui, rotateElement(el.Rotation, corners[i]))
			}
			n := applyGuiNormal(m.gui, rotateElementNormal(el.Rotation, normal))
			if n[2] <= 1e-4 {
				continue
			}

			uv := defaultUV(el, dir)
			if f.UV != nil {
				uv = *f.UV
			}
			tint := color.NRGBA{255, 255, 255, 255}
			if f.TintIndex != nil && *f.TintIndex >= 0 && *f.TintIndex < len(tints) {
				tint = p.tintColor(tints[*f.TintIndex])
			}
			shade := 1.0
			if m.sideLight && (el.Shade == nil || *el.Shade) {
				shade = sideShade(n)
			}

			toScreen := func(v vec3) vec3 {
				return vec3{float64(size)/2 + v[0]*float64(size), float64(size)/2 - v[1]*float64(size), v[2]}
			}
			quads = append(quads, quad{
				p00: toScreen(corners[0]), p10: toScreen(corners[1]), p01: toScreen(corners[2]),
				tex: tex, uv: uv, rotation: ((f.Rotation % 360) + 360) % 360, tint: tint, shade: shade,
				translucent: hasTranslucency(tex),
			})
		}
	}
	if len(quads) == 0 {
		return nil, fmt.Errorf("%w: no visible faces", errUnsupported)
	}

	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	depth := make([]float64, size*size)
	for i := range depth {
		depth[i] = math.Inf(-1)
	}

	// Opaque faces first with a depth buffer, then translucent ones back to
	// front so stained glass and beacon shells show what is behind them.
	var translucent []quad
	for _, q := range quads {
		if q.translucent {
			translucent = append(translucent, q)
			continue
		}
		rasterize(img, depth, q, true)
	}
	sort.Slice(translucent, func(i, j int) bool { return centerDepth(translucent[i]) < centerDepth(translucent[j]) })
	for _, q := range translucent {
		rasterize(img, depth, q, false)
	}
	return downsample(img, supersample), nil
}

// faceCorners returns the top-left, top-right and bottom-left corners of a
// face in block units centered on the origin, matching the UV orientation.
func faceCorners(el element, dir string) ([3]vec3, vec3, bool) {
	f, t := el.From, el.To
	c := func(x, y, z float64) vec3 { return vec3{x/16 - 0.5, y/16 - 0.5, z/16 - 0.5} }
	switch dir {
	case "north":
		return [3]vec3{c(t[0], t[1], f[2]), c(f[0], t[1], f[2]), c(t[0], f[1], f[2])}, vec3{0, 0, -1}, true
	case "south":
		return [3]vec3{c(f[0], t[1], t[2]), c(t[0], t[1], t[2]), c(f[0], f[1], t[2])}, vec3{0, 0, 1}, true
	case "west":
		return [3]vec3{c(f[0], t[1], f[2]), c(f[0], t[1], t[2]), c(f[0], f[1], f[2])}, vec3{-1, 0, 0}, true
	case "east":
		return [3]vec3{c(t[0], t[1], t[2]), c(t[0], t[1], f[2]), c(t[0], f[1], t[2])}, vec3{1, 0, 0}, true
	case "up":
		return [3]vec3{c(f[0], t[1], f[2]), c(t[0], t[1], f[2]), c(f[0], t[1], t[2])}, vec3{0, 1, 0}, true
	case "down":
		return [3]vec3{c(f[0], f[1], t[2]), c(t[0], f[1], t[2]), c(f[0], f[1], f[2])}, vec3{0, -1, 0}, true
	}
	return [3]vec3{}, vec3{}, false
}

func defaultUV(el element, dir string) [4]float64 {
	f, t := el.From, el.To
	switch dir {
	case "up":
		return [4]float64{f[0], f[2], t[0], t[2]}
	case "down":
		return [4]float64{f[0], 16 - t[2], t[0], 16 - f[2]}
	case "north":
		return [4]float64{16 - t[0], 16 - t[1], 16 - f[0], 16 - f[1]}
	case "south":
		return [4]float64{f[0], 16 - t[1], t[0], 16 - f[1]}
	case "west":
		return [4]float64{f[2], 16 - t[1], t[2], 16 - f[1]}
	default:
		return [4]float64{16 - t[2], 16 - t[1], 16 - f[2], 16 - f[1]}
	}
}

func rotateElement(r *elemRotation, v vec3) vec3 {
	if r == nil || r.Angle == 0 {
		return v
	}
	o := vec3{r.Origin[0]/16 - 0.5, r.Origin[1]/16 - 0.5, r.Origin[2]/16 - 0.5}
	d := vec3{v[0] - o[0], v[1] - o[1], v[2] - o[2]}
	d = rotateAxis(d, r.Axis, r.Angle)
	if r.Rescale {
		s := 1 / math.Cos(math.Abs(r.Angle)*math.Pi/180)
		switch r.Axis {
		case "x":
			d[1], d[2] = d[1]*s, d[2]*s
		case "y":
			d[0], d[2] = d[0]*s, d[2]*s
		case "z":
			d[0], d[1] = d[0]*s, d[1]*s
		}
	}
	return vec3{d[0] + o[0], d[1] + o[1], d[2] + o[2]}
}

func rotateElementNormal(r *elemRotation, n vec3) vec3 {
	if r == nil || r.Angle == 0 {
		return n
	}
	return rotateAxis(n, r.Axis, r.Angle)
}

func rotateAxis(v vec3, axis string, deg float64) vec3 {
	a := deg * math.Pi / 180
	s, c := math.Sin(a), math.Cos(a)
	switch axis {
	case "x":
		return vec3{v[0], v[1]*c - v[2]*s, v[1]*s + v[2]*c}
	case "y":
		return vec3{v[0]*c + v[2]*s, v[1], -v[0]*s + v[2]*c}
	default:
		return vec3{v[0]*c - v[1]*s, v[0]*s + v[1]*c, v[2]}
	}
}

// applyGui mirrors ItemTransform: scale, then rotate z, y, x, then translate.
// Translation is stored in 1/16 block units like the model files.
func applyGui(t transform, v vec3) vec3 {
	v = vec3{v[0] * t.scale[0], v[1] * t.scale[1], v[2] * t.scale[2]}
	v = rotateAxis(v, "z", t.rotation[2])
	v = rotateAxis(v, "y", t.rotation[1])
	v = rotateAxis(v, "x", t.rotation[0])
	return vec3{v[0] + t.translation[0]/16, v[1] + t.translation[1]/16, v[2] + t.translation[2]/16}
}

func applyGuiNormal(t transform, n vec3) vec3 {
	n = rotateAxis(n, "z", t.rotation[2])
	n = rotateAxis(n, "y", t.rotation[1])
	return rotateAxis(n, "x", t.rotation[0])
}

// sideShade approximates the inventory lighting: tops are brightest, the
// face turned to the left is lighter than the one turned to the right.
func sideShade(n vec3) float64 {
	if n[1] > 0.7 {
		return 1.0
	}
	if n[1] < -0.7 {
		return 0.5
	}
	if n[0] < -0.2 {
		return 0.8
	}
	if n[0] > 0.2 {
		return 0.6
	}
	return 0.9
}

func centerDepth(q quad) float64 {
	return (q.p10[2] + q.p01[2]) / 2
}

// rasterize fills one parallelogram face. The face is affine in screen
// space, so every pixel maps back to face coordinates with a 2x2 inverse.
func rasterize(img *image.NRGBA, depth []float64, q quad, writeDepth bool) {
	ex := vec3{q.p10[0] - q.p00[0], q.p10[1] - q.p00[1], q.p10[2] - q.p00[2]}
	ey := vec3{q.p01[0] - q.p00[0], q.p01[1] - q.p00[1], q.p01[2] - q.p00[2]}
	det := ex[0]*ey[1] - ex[1]*ey[0]
	if math.Abs(det) < 1e-9 {
		return
	}

	minX := math.Min(math.Min(q.p00[0], q.p10[0]), math.Min(q.p01[0], q.p10[0]+ey[0]))
	maxX := math.Max(math.Max(q.p00[0], q.p10[0]), math.Max(q.p01[0], q.p10[0]+ey[0]))
	minY := math.Min(math.Min(q.p00[1], q.p10[1]), math.Min(q.p01[1], q.p10[1]+ey[1]))
	maxY := math.Max(math.Max(q.p00[1], q.p10[1]), math.Max(q.p01[1], q.p10[1]+ey[1]))

	b := img.Bounds()
	tw, th := q.tex.Bounds().Dx(), q.tex.Bounds().Dy()
	for y := max(int(math.Floor(minY)), 0); y < min(int(math.Ceil(maxY)), b.Dy()); y++ {
		for x := max(int(math.Floor(minX)), 0); x < min(int(math.Ceil(maxX)), b.Dx()); x++ {
			px, py := float64(x)+0.5-q.p00[0], float64(y)+0.5-q.p00[1]
			s := (px*ey[1] - py*ey[0]) / det
			t := (ex[0]*py - ex[1]*px) / det
			if s < 0 || s >= 1 || t < 0 || t >= 1 {
				continue
			}
			z := q.p00[2] + s*ex[2] + t*ey[2]
			idx := y*b.Dx() + x
			if z < depth[idx]-1e-6 {
				continue
			}

			su, sv := s, t
			switch q.rotation {
			case 90:
				su, sv = t, 1-s
			case 180:
				su, sv = 1-s, 1-t
			case 270:
				su, sv = 1-t, s
			}
			u := (q.uv[0] + su*(q.uv[2]-q.uv[0])) / 16
			v := (q.uv[1] + sv*(q.uv[3]-q.uv[1])) / 16
			tx := min(max(int(math.Floor(u*float64(tw))), 0), tw-1)
			ty := min(max(int(math.Floor(v*float64(th))), 0), th-1)
			c := q.tex.NRGBAAt(tx, ty)
			if c.A == 0 {
				continue
			}
			blend(img, x, y, multiply(c, q.tint, q.shade))
			if writeDepth {
				depth[idx] = z
			}
		}
	}
}

// ==== pixel helpers ====

func multiply(c, tint color.NRGBA, shade float64) color.NRGBA {
	f := func(a, b uint8) uint8 {
		return uint8(math.Round(float64(a) * float64(b) / 255 * shade))
	}
	return color.NRGBA{f(c.R, tint.R), f(c.G, tint.G), f(c.B, tint.B), c.A}
}

func blend(img *image.NRGBA, x, y int, src color.NRGBA) {
	if src.A == 255 {
		img.SetNRGBA(x, y, src)
		return
	}
	dst := img.NRGBAAt(x, y)
	sa, da := float64(src.A)/255, float64(dst.A)/255
	oa := sa + da*(1-sa)
	if oa == 0 {
		return
	}
	mix := func(s, d uint8) uint8 {
		return uint8(math.Round((float64(s)*sa + float64(d)*da*(1-sa)) / oa))
	}
	img.SetNRGBA(x, y, color.NRGBA{mix(src.R, dst.R), mix(src.G, dst.G), mix(src.B, dst.B), uint8(math.Round(oa * 255))})
}

func over(dst, src *image.NRGBA) {
	b := dst.Bounds()
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			if c := src.NRGBAAt(x, y); c.A > 0 {
				blend(dst, x, y, c)
			}
		}
	}
}

// downsample averages factor x factor blocks with premultiplied alpha so
// transparent edges do not darken.
func downsample(src *image.NRGBA, factor int) *image.NRGBA {
	b := src.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx()/factor, b.Dy()/factor))
	n := float64(factor * factor)
	for y := 0; y < out.Bounds().Dy(); y++ {
		for x := 0; x < out.Bounds().Dx(); x++ {
			var r, g, bl, a float64
			for dy := 0; dy < factor; dy++ {
				for dx := 0; dx < factor; dx++ {
					c := src.NRGBAAt(x*factor+dx, y*factor+dy)
					ca := float64(c.A)
					r += float64(c.R) * ca
					g += float64(c.G) * ca
					bl += float64(c.B) * ca
					a += ca
				}
			}
			if a == 0 {
				continue
			}
			out.SetNRGBA(x, y, color.NRGBA{
				R: uint8(math.Round(r / a)), G: uint8(math.Round(g / a)), B: uint8(math.Round(bl / a)),
				A: uint8(math.Round(a / n)),
			})
		}
	}
	return out
}

func hasTranslucency(img *image.NRGBA) bool {
	for i := 3; i < len(img.Pix); i += 4 {
		if a := img.Pix[i]; a > 0 && a < 255 {
			return true
		}
	}
	return false
}

func isEmpty(img *image.NRGBA) bool {
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] > 0 {
			return false
		}
	}
	return true
}
