package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeyHashEnvOrderIndependent(t *testing.T) {
	envA := map[string]string{"A": "1", "B": "2"}
	envB := map[string]string{"B": "2", "A": "1"}
	k1, err := KeyHash("", "RUN echo", "/app", envA, "RUN", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	k2, err := KeyHash("", "RUN echo", "/app", envB, "RUN", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if k1 != k2 {
		t.Fatalf("keys differ: %s vs %s", k1, k2)
	}
}

func TestKeyHashRawWhitespaceMatters(t *testing.T) {
	ctx := t.TempDir()
	k1, err := KeyHash("", "COPY . /app", "/", map[string]string{}, "COPY", []string{".", "/app"}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := KeyHash("", "COPY    .    /app", "/", map[string]string{}, "COPY", []string{".", "/app"}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if k1 == k2 {
		t.Fatal("expected different keys for different raw instruction text")
	}
}

func TestKeyHashWorkdirMatters(t *testing.T) {
	ctx := t.TempDir()
	if err := os.WriteFile(filepath.Join(ctx, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	k1, err := KeyHash("", "COPY f.txt /w/", "/a", map[string]string{}, "COPY", []string{"f.txt", "/w/"}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := KeyHash("", "COPY f.txt /w/", "/b", map[string]string{}, "COPY", []string{"f.txt", "/w/"}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if k1 == k2 {
		t.Fatal("expected different keys for different workdir")
	}
}

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	key := "deadbeef" + "deadbeef" + "deadbeef" + "deadbeef" + "deadbeef" + "deadbeef" + "deadbeef" + "deadbeef"
	dig := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, hit := s.Check(key); hit {
		t.Fatal("unexpected hit")
	}
	if err := s.Store(key, dig); err != nil {
		t.Fatal(err)
	}
	got, hit := s.Check(key)
	if !hit || got != dig {
		t.Fatalf("got %q hit=%v", got, hit)
	}
}
