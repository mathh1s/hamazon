package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mathh1s/hamazon/internal/worldmap"
)

func (s *Server) handleMapInfo(w http.ResponseWriter, r *http.Request) {
	if s.worldMap == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	x, z := s.worldMap.Spawn()
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":  true,
		"version":  s.worldMap.Version(),
		"tileSize": worldmap.TileSize,
		"scales":   worldmap.Scales,
		"spawn":    map[string]int{"x": x, "z": z},
	})
}

// handleMapTile serves /api/map/{dim}/{scale}/{x}_{z}.png. Tiles depend
// only on the seed, so browsers may keep them; the version query changes
// with the seed.
func (s *Server) handleMapTile(w http.ResponseWriter, r *http.Request) {
	if s.worldMap == nil {
		http.NotFound(w, r)
		return
	}
	if !s.mapLimit.Allow(s.clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many map requests")
		return
	}
	dim, ok := worldmap.ParseDimension(r.PathValue("dim"))
	scale, err1 := strconv.Atoi(r.PathValue("scale"))
	name, isPNG := strings.CutSuffix(r.PathValue("tile"), ".png")
	xs, zs, found := strings.Cut(name, "_")
	tx, err2 := strconv.Atoi(xs)
	tz, err3 := strconv.Atoi(zs)
	if !ok || err1 != nil || !isPNG || !found || err2 != nil || err3 != nil {
		http.NotFound(w, r)
		return
	}
	data, err := s.worldMap.Tile(dim, scale, tx, tz)
	if errors.Is(err, worldmap.ErrBadTile) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=604800, immutable")
	_, _ = w.Write(data)
}

func (s *Server) handleMapVillages(w http.ResponseWriter, r *http.Request) {
	if s.worldMap == nil {
		http.NotFound(w, r)
		return
	}
	if !s.mapLimit.Allow(s.clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many map requests")
		return
	}
	q := r.URL.Query()
	var v [4]int
	for i, k := range []string{"x0", "z0", "x1", "z1"} {
		n, err := strconv.Atoi(q.Get(k))
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "Missing area")
			return
		}
		v[i] = n
	}
	list, err := s.worldMap.Villages(v[0], v[1], v[2], v[3])
	if errors.Is(err, worldmap.ErrBadTile) {
		writeError(w, http.StatusBadRequest, "bad_request", "Area too large")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"villages": list})
}
