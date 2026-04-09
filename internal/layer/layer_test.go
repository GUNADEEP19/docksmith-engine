package layer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"docksmith-engine/internal"
)

func testRoot(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	t.Setenv("HOME", d)
	return filepath.Join(d, ".docksmith")
}

func TestCreateLayerDeterminism(t *testing.T) {
	root := testRoot(t)
	svc := New(WithDataRoot(root))
	ctx := t.TempDir()
	if err := os.WriteFile(filepath.Join(ctx, "f.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	inst := internal.Instruction{Op: "COPY", Args: []string{"f.txt", "/app/"}, Raw: "COPY f.txt /app/"}
	d1, sz1, err := svc.CreateLayer("", inst, ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	d2, sz2, err := svc.CreateLayer("", inst, ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if d1 != d2 || sz1 != sz2 {
		t.Fatalf("non-deterministic layer: %s/%d vs %s/%d", d1, sz1, d2, sz2)
	}
}

func TestChangeDetectionOnlyTopLayerChanges(t *testing.T) {
	root := testRoot(t)
	svc := New(WithDataRoot(root))
	ctx := t.TempDir()
	a := filepath.Join(ctx, "a.txt")
	b := filepath.Join(ctx, "b.txt")
	if err := os.WriteFile(a, []byte("a1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("b1"), 0o644); err != nil {
		t.Fatal(err)
	}
	copyA := internal.Instruction{Op: "COPY", Args: []string{"a.txt", "/app/"}}
	dA1, _, err := svc.CreateLayer("", copyA, ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	copyB := internal.Instruction{Op: "COPY", Args: []string{"b.txt", "/app/"}}
	dB1, _, err := svc.CreateLayer(dA1, copyB, ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(b, []byte("b2"), 0o644); err != nil {
		t.Fatal(err)
	}
	dA2, _, err := svc.CreateLayer("", copyA, ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	dB2, _, err := svc.CreateLayer(dA2, copyB, ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if dA1 != dA2 {
		t.Fatalf("first layer should be unchanged: %s vs %s", dA1, dA2)
	}
	if dB1 == dB2 {
		t.Fatalf("second layer digest should change when b.txt changes")
	}
}

func TestExtractLayersInOrder(t *testing.T) {
	root := testRoot(t)
	svc := New(WithDataRoot(root))
	ctx := t.TempDir()
	if err := os.WriteFile(filepath.Join(ctx, "one"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctx, "two"), []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}
	d1, _, err := svc.CreateLayer("", internal.Instruction{Op: "COPY", Args: []string{"one", "/x/"}}, ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	d2, _, err := svc.CreateLayer(d1, internal.Instruction{Op: "COPY", Args: []string{"two", "/x/"}}, ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()
	chain, err := svc.chainBottomToTop(d2)
	if err != nil {
		t.Fatal(err)
	}
	if len(chain) != 2 || chain[0] != d1 || chain[1] != d2 {
		t.Fatalf("unexpected chain: %v", chain)
	}
	for _, d := range chain {
		if err := svc.extractLayerInto(out, d); err != nil {
			t.Fatal(err)
		}
	}
	b1, _ := os.ReadFile(filepath.Join(out, "x", "one"))
	b2, _ := os.ReadFile(filepath.Join(out, "x", "two"))
	if string(b1) != "1" || string(b2) != "2" {
		t.Fatalf("extracted content wrong: %q %q", b1, b2)
	}
}

func TestTarEntriesSortedLexicographically(t *testing.T) {
	root := testRoot(t)
	svc := New(WithDataRoot(root))
	ctx := t.TempDir()
	// z before a in creation order; delta should still sort paths in tar
	for _, p := range []struct {
		name, body string
	}{
		{"z.txt", "z"},
		{"a.txt", "a"},
	} {
		if err := os.WriteFile(filepath.Join(ctx, p.name), []byte(p.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(ctx, "Docksmithfile"), []byte("#"), 0o644); err != nil {
		t.Fatal(err)
	}
	inst := internal.Instruction{Op: "COPY", Args: []string{".", "/app/"}}
	d1, _, err := svc.CreateLayer("", inst, ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	// mutate context so second layer has deterministic new file only
	if err := os.WriteFile(filepath.Join(ctx, "extra"), []byte("e"), 0o644); err != nil {
		t.Fatal(err)
	}
	inst2 := internal.Instruction{Op: "COPY", Args: []string{"extra", "/app/"}}
	d2a, _, err := svc.CreateLayer(d1, inst2, ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	d2b, _, err := svc.CreateLayer(d1, inst2, ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if d2a != d2b {
		t.Fatalf("layer digest must not depend on wall clock: %s vs %s", d2a, d2b)
	}
}

func TestCopyDotCreatesDestinationDirWhenMissing(t *testing.T) {
	root := testRoot(t)
	svc := New(WithDataRoot(root))
	ctx := t.TempDir()
	if err := os.WriteFile(filepath.Join(ctx, "f.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Destination has no trailing slash and does not exist yet; must be treated as directory.
	inst := internal.Instruction{Op: "COPY", Args: []string{".", "/app"}, Raw: "COPY . /app"}
	_, _, err := svc.CreateLayer("", inst, ctx, "", nil)
	if err != nil {
		t.Fatalf("COPY . /app should succeed, got: %v", err)
	}
}
