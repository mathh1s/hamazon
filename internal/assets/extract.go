package assets

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ManifestItem struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Kind iconKind `json:"kind"`
}

type Manifest struct {
	Version     string         `json:"version"`
	GeneratedAt time.Time      `json:"generatedAt"`
	Items       []ManifestItem `json:"items"`
	Font        bool           `json:"font"`
}

type Report struct {
	Manifest Manifest
	Skipped  map[string]string
}

// Extract renders every item icon, the GUI sprites and the game font from a
// client jar into dir. It builds into a sibling directory and swaps it in at
// the end, so a running server never serves a half written set.
func Extract(jarPath, dir string) (Report, error) {
	zr, err := zip.OpenReader(jarPath)
	if err != nil {
		return Report{}, fmt.Errorf("open jar: %w", err)
	}
	defer zr.Close()
	p := openPack(&zr.Reader)
	if !p.has("assets/minecraft/textures/gui/container/generic_54.png") {
		return Report{}, errors.New("this does not look like a Minecraft client jar")
	}

	staging := dir + ".new"
	if err := os.RemoveAll(staging); err != nil {
		return Report{}, err
	}
	for _, sub := range []string{"item", "gui", "font"} {
		if err := os.MkdirAll(filepath.Join(staging, sub), 0o755); err != nil {
			return Report{}, err
		}
	}

	report := Report{Skipped: map[string]string{}}
	report.Manifest.GeneratedAt = time.Now().UTC()
	report.Manifest.Version = p.version()
	names := p.englishNames()

	var ids []string
	for name := range p.files {
		if strings.HasPrefix(name, "assets/minecraft/items/") && strings.HasSuffix(name, ".json") {
			ids = append(ids, strings.TrimSuffix(path.Base(name), ".json"))
		}
	}
	sort.Strings(ids)

	for _, id := range ids {
		var def itemDefinition
		if err := p.readJSON("assets/minecraft/items/"+id+".json", &def); err != nil {
			report.Skipped[id] = err.Error()
			continue
		}
		refs, err := resolveItem(def.Model)
		if err != nil {
			report.Skipped[id] = err.Error()
			continue
		}
		img, kind, err := p.renderItem(refs)
		if err != nil {
			report.Skipped[id] = err.Error()
			continue
		}
		if err := writePNG(filepath.Join(staging, "item", id+".png"), img); err != nil {
			return report, err
		}
		name := names[id]
		if name == "" {
			name = id
		}
		report.Manifest.Items = append(report.Manifest.Items, ManifestItem{ID: id, Name: name, Kind: kind})
	}

	gui, err := p.guiImages()
	if err != nil {
		return report, err
	}
	for name, img := range gui {
		if err := writePNG(filepath.Join(staging, "gui", name+".png"), img); err != nil {
			return report, err
		}
	}

	glyphs, err := p.collectGlyphs()
	if err != nil {
		report.Skipped["font"] = err.Error()
	} else {
		font, err := buildTrueType(glyphs, "Hamazon Minecraft")
		if err != nil {
			return report, err
		}
		if err := os.WriteFile(filepath.Join(staging, "font", "minecraft.ttf"), font, 0o644); err != nil {
			return report, err
		}
		report.Manifest.Font = true
	}

	manifest, err := json.Marshal(report.Manifest)
	if err != nil {
		return report, err
	}
	if err := os.WriteFile(filepath.Join(staging, "manifest.json"), manifest, 0o644); err != nil {
		return report, err
	}

	old := dir + ".old"
	_ = os.RemoveAll(old)
	if _, err := os.Stat(dir); err == nil {
		if err := os.Rename(dir, old); err != nil {
			return report, err
		}
	}
	if err := os.Rename(staging, dir); err != nil {
		return report, err
	}
	_ = os.RemoveAll(old)
	return report, nil
}

func (p *pack) version() string {
	var v struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	if err := p.readJSON("version.json", &v); err != nil {
		return "unknown"
	}
	if v.Name != "" {
		return v.Name
	}
	return v.ID
}

func (p *pack) englishNames() map[string]string {
	var lang map[string]string
	_ = p.readJSON("assets/minecraft/lang/en_us.json", &lang)
	out := map[string]string{}
	for key, value := range lang {
		for _, prefix := range []string{"block.minecraft.", "item.minecraft."} {
			if id, ok := strings.CutPrefix(key, prefix); ok && !strings.Contains(id, ".") {
				if _, set := out[id]; !set || prefix == "item.minecraft." {
					out[id] = value
				}
			}
		}
	}
	return out
}

func writePNG(name string, img image.Image) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
