package image

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"docksmith-engine/internal"
)

// Store implements internal.ImageStore.
type Store struct {
	root string
}

// Option configures Store.
type Option func(*Store)

// WithDataRoot sets ~/.docksmith equivalent root.
func WithDataRoot(root string) Option {
	return func(s *Store) {
		if strings.TrimSpace(root) != "" {
			s.root = root
		}
	}
}

// NewStore creates an image store under ~/.docksmith/images.
func NewStore(opts ...Option) *Store {
	h, err := os.UserHomeDir()
	if err != nil {
		h = "."
	}
	s := &Store{root: filepath.Join(h, ".docksmith")}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Store) imagesDir() string {
	return filepath.Join(s.root, "images")
}

func manifestFilePath(imagesDir, name, tag string) string {
	safe := strings.ReplaceAll(name, "/", "_") + "__" + strings.ReplaceAll(tag, "/", "_") + ".json"
	return filepath.Join(imagesDir, safe)
}

// Save writes the manifest JSON with computed digest.
func (s *Store) Save(image internal.Image) error {
	if strings.TrimSpace(image.Name) == "" || strings.TrimSpace(image.Tag) == "" {
		return fmt.Errorf("image: name and tag are required")
	}
	if err := os.MkdirAll(s.imagesDir(), 0o755); err != nil {
		return fmt.Errorf("image: mkdir: %w", err)
	}
	path := manifestFilePath(s.imagesDir(), image.Name, image.Tag)

	// Compute deterministic digest (independent of Created).
	_, digest, err := serializeManifest(image)
	if err != nil {
		return err
	}
	image.Digest = digest

	// created semantics:
	// 1) First build: set CreatedAt to now.
	// 2) Rebuild with identical content (digest unchanged): preserve existing CreatedAt.
	existing, err := os.ReadFile(path)
	if err == nil {
		oldImg, parseErr := parseManifestBytes(existing)
		if parseErr == nil && oldImg.Digest == digest && !oldImg.CreatedAt.IsZero() {
			image.CreatedAt = oldImg.CreatedAt
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if image.CreatedAt.IsZero() {
		image.CreatedAt = time.Now().UTC()
	}

	finalJSON, _, err := serializeManifest(image)
	if err != nil {
		return err
	}
	return os.WriteFile(path, finalJSON, 0o644)
}

// Load reads a manifest by name:tag.
func (s *Store) Load(nameTag string) (internal.Image, error) {
	parts := strings.SplitN(nameTag, ":", 2)
	if len(parts) != 2 {
		return internal.Image{}, fmt.Errorf("image: invalid name:tag %q", nameTag)
	}
	path := manifestFilePath(s.imagesDir(), strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return internal.Image{}, fmt.Errorf("image %s not found", nameTag)
		}
		return internal.Image{}, err
	}
	return parseManifestBytes(data)
}

// List returns all stored images.
func (s *Store) List() ([]internal.Image, error) {
	entries, err := os.ReadDir(s.imagesDir())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []internal.Image
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.imagesDir(), e.Name()))
		if err != nil {
			continue
		}
		img, err := parseManifestBytes(data)
		if err != nil {
			continue
		}
		out = append(out, img)
	}
	sort.Slice(out, func(i, j int) bool {
		ki := out[i].Name + ":" + out[i].Tag
		kj := out[j].Name + ":" + out[j].Tag
		return ki < kj
	})
	return out, nil
}

func parseManifestBytes(data []byte) (internal.Image, error) {
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return internal.Image{}, fmt.Errorf("image: decode manifest: %w", err)
	}
	env := map[string]string{}
	for _, e := range m.Config.Env {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			env[k] = v
		}
	}
	layers := make([]internal.ImageLayer, 0, len(m.Layers))
	for _, L := range m.Layers {
		layers = append(layers, internal.ImageLayer{
			Digest:    L.Digest,
			Size:      L.Size,
			CreatedBy: L.CreatedBy,
		})
	}
	var created time.Time
	if strings.TrimSpace(m.Created) != "" {
		t, err := time.Parse(time.RFC3339, m.Created)
		if err == nil {
			created = t
		}
	}
	return internal.Image{
		Name:      m.Name,
		Tag:       m.Tag,
		Digest:    m.Digest,
		CreatedAt: created,
		Config: internal.ImageConfig{
			Cmd:        append([]string(nil), m.Config.Cmd...),
			Env:        env,
			WorkingDir: m.Config.WorkingDir,
		},
		Layers: layers,
	}, nil
}

// Remove deletes the manifest and layer blobs referenced by it.
func (s *Store) Remove(nameTag string) error {
	parts := strings.SplitN(nameTag, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("image: invalid name:tag %q", nameTag)
	}
	path := manifestFilePath(s.imagesDir(), strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("image %s not found", nameTag)
		}
		return err
	}
	img, err := parseManifestBytes(data)
	if err != nil {
		return err
	}
	for _, L := range img.Layers {
		s.removeLayerFiles(L.Digest)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Store) removeLayerFiles(digest string) {
	hex := strings.TrimPrefix(strings.TrimSpace(digest), "sha256:")
	if len(hex) != 64 {
		return
	}
	_ = os.Remove(filepath.Join(s.root, "layers", fmt.Sprintf("sha256_%s.tar", hex)))
	_ = os.Remove(filepath.Join(s.root, "layers", fmt.Sprintf("sha256_%s.parent", hex)))
}
