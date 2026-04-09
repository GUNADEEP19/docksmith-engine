package builder

import (
	"os"
	"path/filepath"
	"testing"

	"docksmith-engine/internal/image"
	"docksmith-engine/internal/layer"
	"docksmith-engine/internal/parser"
)

func TestBuildLayerDigestsDeterministic(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".docksmith")

	ctx := t.TempDir()
	if err := os.WriteFile(filepath.Join(ctx, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	df := `FROM scratch
COPY a.txt /app/
COPY a.txt /app2/
`
	if err := os.WriteFile(filepath.Join(ctx, "Docksmithfile"), []byte(df), 0o644); err != nil {
		t.Fatal(err)
	}

	p := parser.New()
	lyr := layer.New(layer.WithDataRoot(root))
	b := New(lyr, WithDataRoot(root))
	inst, err := p.Parse(filepath.Join(ctx, "Docksmithfile"))
	if err != nil {
		t.Fatal(err)
	}
	img1, err := b.Build(inst, "t:1", ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	img2, err := b.Build(inst, "t:1", ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(img1.Layers) != 2 || len(img2.Layers) != 2 {
		t.Fatalf("expected 2 layers, got %d and %d", len(img1.Layers), len(img2.Layers))
	}
	for i := range img1.Layers {
		if img1.Layers[i].Digest != img2.Layers[i].Digest {
			t.Fatalf("layer %d digest differs: %s vs %s", i, img1.Layers[i].Digest, img2.Layers[i].Digest)
		}
	}
}

func TestBuildCopyCacheInvalidatesWhenSourceChanges(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".docksmith")

	ctx := t.TempDir()
	if err := os.WriteFile(filepath.Join(ctx, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	df := `FROM scratch
COPY a.txt /app/
COPY a.txt /app2/
`
	if err := os.WriteFile(filepath.Join(ctx, "Docksmithfile"), []byte(df), 0o644); err != nil {
		t.Fatal(err)
	}

	p := parser.New()
	lyr := layer.New(layer.WithDataRoot(root))
	b := New(lyr, WithDataRoot(root))
	inst, err := p.Parse(filepath.Join(ctx, "Docksmithfile"))
	if err != nil {
		t.Fatal(err)
	}
	img1, err := b.Build(inst, "t:1", ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(img1.Layers) != 2 {
		t.Fatalf("want 2 layers, got %d", len(img1.Layers))
	}

	if err := os.WriteFile(filepath.Join(ctx, "a.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	img2, err := b.Build(inst, "t:1", ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if img2.Layers[0].Digest == img1.Layers[0].Digest {
		t.Fatal("expected COPY layer digest to change when source file changes")
	}
	if img2.Layers[1].Digest == img1.Layers[1].Digest {
		t.Fatal("expected subsequent layer digest to change when source file changes")
	}
}

func TestBuildSaveLoadRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".docksmith")

	ctx := t.TempDir()
	if err := os.WriteFile(filepath.Join(ctx, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctx, "Docksmithfile"), []byte("FROM scratch\nCOPY f.txt /w/\nCMD [\"sh\"]"), 0o644); err != nil {
		t.Fatal(err)
	}

	lyr := layer.New(layer.WithDataRoot(root))
	b := New(lyr, WithDataRoot(root))
	st := image.NewStore(image.WithDataRoot(root))
	inst, err := parser.New().Parse(filepath.Join(ctx, "Docksmithfile"))
	if err != nil {
		t.Fatal(err)
	}
	img, err := b.Build(inst, "app:latest", ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Save(img); err != nil {
		t.Fatal(err)
	}
	loaded, err := st.Load("app:latest")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Digest != img.Digest || len(loaded.Layers) != len(img.Layers) {
		t.Fatalf("round trip mismatch digest=%q vs %q layers=%d vs %d", loaded.Digest, img.Digest, len(loaded.Layers), len(img.Layers))
	}
}
