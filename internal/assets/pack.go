package assets

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"io"
	"strings"
)

// pack gives read access to the assets inside a client jar or resource pack zip.
type pack struct {
	files    map[string]*zip.File
	textures map[string]*image.NRGBA
}

func openPack(r *zip.Reader) *pack {
	p := &pack{files: map[string]*zip.File{}, textures: map[string]*image.NRGBA{}}
	for _, f := range r.File {
		p.files[f.Name] = f
	}
	return p
}

func (p *pack) read(name string) ([]byte, error) {
	f, ok := p.files[name]
	if !ok {
		return nil, fmt.Errorf("%s: %w", name, errMissing)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func (p *pack) has(name string) bool {
	_, ok := p.files[name]
	return ok
}

func (p *pack) readJSON(name string, v any) error {
	data, err := p.read(name)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// resourcePath turns "minecraft:block/stone" into its file path below assets/.
func resourcePath(id, kind, ext string) string {
	ns, path := "minecraft", id
	if i := strings.IndexByte(id, ':'); i >= 0 {
		ns, path = id[:i], id[i+1:]
	}
	return "assets/" + ns + "/" + kind + "/" + path + ext
}

// texture loads a texture by resource id. Animated textures are vertical
// strips of frames, only the first frame is kept since icons are static.
func (p *pack) texture(id string) (*image.NRGBA, error) {
	if img, ok := p.textures[id]; ok {
		return img, nil
	}
	data, err := p.read(resourcePath(id, "textures", ".png"))
	if err != nil {
		return nil, err
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", id, err)
	}
	b := src.Bounds()
	h := b.Dy()
	if b.Dy() > b.Dx() && b.Dy()%b.Dx() == 0 {
		h = b.Dx()
	}
	img := image.NewNRGBA(image.Rect(0, 0, b.Dx(), h))
	draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)
	p.textures[id] = img
	return img, nil
}
