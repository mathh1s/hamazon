package assets

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const versionManifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"

// formatVersion is bumped whenever the extractor output changes, so existing
// installs re-extract on the next start without anyone running a command.
const formatVersion = 6

type State string

const (
	StateReady       State = "ready"
	StateDownloading State = "downloading"
	StateFailed      State = "failed"
	StateDisabled    State = "disabled"
)

type Status struct {
	State   State  `json:"state"`
	Version string `json:"version"`
	Error   string `json:"error,omitempty"`
}

// Manager keeps the extracted assets in step with the configured Minecraft
// version by downloading the official client jar when needed.
type Manager struct {
	dir     string
	version string
	client  *http.Client
	log     *slog.Logger

	mu     sync.Mutex
	status Status
}

func NewManager(dir, version string, log *slog.Logger) *Manager {
	return &Manager{
		dir:     dir,
		version: version,
		client:  &http.Client{Timeout: 10 * time.Minute},
		log:     log,
		status:  Status{State: StateDownloading, Version: version},
	}
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *Manager) Disable() {
	state := StateDisabled
	if Current(m.dir, m.version) {
		state = StateReady
	}
	m.setStatus(Status{State: state, Version: m.version})
}

func (m *Manager) setStatus(s Status) {
	m.mu.Lock()
	m.status = s
	m.mu.Unlock()
}

// Current reports whether dir already holds assets for this version and
// extractor format.
func Current(dir, version string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return false
	}
	var m Manifest
	if json.Unmarshal(data, &m) != nil {
		return false
	}
	return m.Version == version && m.Format == formatVersion
}

// Run makes sure the assets exist, retrying with backoff until it works or
// ctx ends. The shop keeps serving meanwhile, it just shows placeholder
// icons until the swap.
func (m *Manager) Run(ctx context.Context) {
	if Current(m.dir, m.version) {
		m.setStatus(Status{State: StateReady, Version: m.version})
		return
	}
	delay := 30 * time.Second
	for {
		err := m.download(ctx)
		if err == nil {
			m.setStatus(Status{State: StateReady, Version: m.version})
			return
		}
		if ctx.Err() != nil {
			return
		}
		m.log.Error("asset download failed", "version", m.version, "err", err, "retry_in", delay)
		m.setStatus(Status{State: StateFailed, Version: m.version, Error: err.Error()})
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(delay*2, time.Hour)
	}
}

func (m *Manager) download(ctx context.Context) error {
	m.setStatus(Status{State: StateDownloading, Version: m.version})
	start := time.Now()
	m.log.Info("downloading minecraft assets", "version", m.version)

	jar, err := DownloadClientJar(ctx, m.client, m.version, filepath.Dir(m.dir))
	if err != nil {
		return err
	}
	defer os.Remove(jar)

	sounds, err := DownloadSounds(ctx, m.client, m.version)
	if err != nil {
		m.log.Warn("sounds unavailable, the shop falls back to synthesized ones", "err", err)
	}

	report, err := Extract(jar, m.dir, sounds)
	if err != nil {
		return err
	}
	m.log.Info("assets ready", "version", report.Manifest.Version, "items", len(report.Manifest.Items),
		"skipped", len(report.Skipped), "took", time.Since(start).Round(time.Second))
	return nil
}

type versionMeta struct {
	Downloads struct {
		Client struct {
			SHA1 string `json:"sha1"`
			URL  string `json:"url"`
		} `json:"client"`
	} `json:"downloads"`
	AssetIndex struct {
		URL string `json:"url"`
	} `json:"assetIndex"`
}

func fetchVersionMeta(ctx context.Context, client *http.Client, version string) (versionMeta, error) {
	var meta versionMeta
	var manifest struct {
		Versions []struct {
			ID  string `json:"id"`
			URL string `json:"url"`
		} `json:"versions"`
	}
	if err := getJSON(ctx, client, versionManifestURL, &manifest); err != nil {
		return meta, fmt.Errorf("version list: %w", err)
	}
	var versionURL string
	for _, v := range manifest.Versions {
		if v.ID == version {
			versionURL = v.URL
			break
		}
	}
	if versionURL == "" {
		return meta, fmt.Errorf("minecraft version %q does not exist, check HAMAZON_MC_VERSION", version)
	}
	if err := getJSON(ctx, client, versionURL, &meta); err != nil {
		return meta, fmt.Errorf("version details: %w", err)
	}
	return meta, nil
}

// DownloadClientJar fetches the client jar of a version from Mojang into
// tmpDir and checks it against the published SHA-1.
func DownloadClientJar(ctx context.Context, client *http.Client, version, tmpDir string) (string, error) {
	meta, err := fetchVersionMeta(ctx, client, version)
	if err != nil {
		return "", err
	}
	if meta.Downloads.Client.URL == "" {
		return "", fmt.Errorf("version %s has no client download", version)
	}

	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(tmpDir, "client-*.jar")
	if err != nil {
		return "", err
	}
	path := f.Name()
	fail := func(err error) (string, error) {
		f.Close()
		os.Remove(path)
		return "", err
	}

	resp, err := get(ctx, client, meta.Downloads.Client.URL)
	if err != nil {
		return fail(fmt.Errorf("client jar: %w", err))
	}
	defer resp.Body.Close()
	h := sha1.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		return fail(fmt.Errorf("client jar: %w", err))
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != meta.Downloads.Client.SHA1 {
		return fail(errors.New("client jar checksum does not match, download was corrupted"))
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// uiSounds maps the names the frontend plays to files in the asset index.
// Sounds are not part of the client jar, the launcher fetches them by hash.
var uiSounds = map[string]string{
	"click":       "minecraft/sounds/random/click.ogg",
	"pop":         "minecraft/sounds/random/pop.ogg",
	"levelup":     "minecraft/sounds/random/levelup.ogg",
	"orb":         "minecraft/sounds/random/orb.ogg",
	"chestopen":   "minecraft/sounds/random/chestopen.ogg",
	"chestclosed": "minecraft/sounds/random/chestclosed.ogg",
}

// DownloadSounds fetches the UI sounds of a version and checks each one
// against the SHA-1 it is stored under.
func DownloadSounds(ctx context.Context, client *http.Client, version string) (map[string][]byte, error) {
	meta, err := fetchVersionMeta(ctx, client, version)
	if err != nil {
		return nil, err
	}
	var index struct {
		Objects map[string]struct {
			Hash string `json:"hash"`
		} `json:"objects"`
	}
	if err := getJSON(ctx, client, meta.AssetIndex.URL, &index); err != nil {
		return nil, fmt.Errorf("asset index: %w", err)
	}
	out := map[string][]byte{}
	for name, path := range uiSounds {
		obj, ok := index.Objects[path]
		if !ok || len(obj.Hash) < 2 {
			continue
		}
		resp, err := get(ctx, client, "https://resources.download.minecraft.net/"+obj.Hash[:2]+"/"+obj.Hash)
		if err != nil {
			return out, err
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if err != nil {
			return out, err
		}
		sum := sha1.Sum(data)
		if hex.EncodeToString(sum[:]) != obj.Hash {
			return out, fmt.Errorf("sound %s checksum does not match", name)
		}
		out[name] = data
	}
	return out, nil
}

func get(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Hamazon (private Minecraft shop)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s returned %d", url, resp.StatusCode)
	}
	return resp, nil
}

func getJSON(ctx context.Context, client *http.Client, url string, v any) error {
	resp, err := get(ctx, client, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(v)
}
