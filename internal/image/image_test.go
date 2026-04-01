package image

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"docksmith-engine/internal"
)

func testImageRoot(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	t.Setenv("HOME", d)
	return filepath.Join(d, ".docksmith")
}

func TestManifestDigestFieldCorrect(t *testing.T) {
	root := testImageRoot(t)
	s := NewStore(WithDataRoot(root))
	img := internal.Image{
		Name:      "myapp",
		Tag:       "latest",
		CreatedAt: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		Config: internal.ImageConfig{
			Cmd:        []string{"echo", "hi"},
			Env:        map[string]string{"A": "1"},
			WorkingDir: "/app",
		},
		Layers: []internal.ImageLayer{
			{Digest: "sha256:" + strings.Repeat("a", 64), Size: 10, CreatedBy: "COPY . /app"},
		},
	}
	dig, err := ImageManifestDigest(img)
	if err != nil {
		t.Fatal(err)
	}
	img.Digest = dig
	if err := s.Save(img); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Load("myapp:latest")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Digest != dig {
		t.Fatalf("loaded digest mismatch: %q vs %q", loaded.Digest, dig)
	}
	d2, err := ImageManifestDigest(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if d2 != loaded.Digest {
		t.Fatalf("recomputed manifest digest wrong: %q vs %q", d2, loaded.Digest)
	}
	if len(loaded.Layers) != 1 || loaded.Layers[0].CreatedBy != "COPY . /app" {
		t.Fatalf("layers: %+v", loaded.Layers)
	}
}

func TestListAndRemove(t *testing.T) {
	root := testImageRoot(t)
	s := NewStore(WithDataRoot(root))
	layersDir := filepath.Join(root, "layers")
	if err := os.MkdirAll(layersDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lhex := strings.Repeat("c", 64)
	tarPath := filepath.Join(layersDir, "sha256_"+lhex+".tar")
	if err := os.WriteFile(tarPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	parentPath := filepath.Join(layersDir, "sha256_"+lhex+".parent")
	if err := os.WriteFile(parentPath, []byte(`{"parent":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	img := internal.Image{
		Name:      "x",
		Tag:       "y",
		CreatedAt: time.Unix(0, 0).UTC(),
		Layers: []internal.ImageLayer{
			{Digest: "sha256:" + lhex, Size: 1, CreatedBy: "RUN echo"},
		},
	}
	d, err := ImageManifestDigest(img)
	if err != nil {
		t.Fatal(err)
	}
	img.Digest = d
	if err := s.Save(img); err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v err=%v", list, err)
	}
	if err := s.Remove("x:y"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tarPath); !os.IsNotExist(err) {
		t.Fatalf("layer tar should be removed: %v", err)
	}
	if _, err := os.Stat(parentPath); !os.IsNotExist(err) {
		t.Fatalf("parent should be removed: %v", err)
	}
}
