package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"docksmith-engine/internal"
)

type mockParser struct{}

type mockBuilder struct{}

type mockCache struct{}

type mockLayer struct{}

type mockImageStore struct {
	mu     sync.Mutex
	images map[string]internal.Image
	dbPath string
	loaded bool
}

type mockRuntime struct{}

type imageRecord struct {
	Name   string            `json:"name"`
	Tag    string            `json:"tag"`
	Digest string            `json:"digest"`
	Cmd    []string          `json:"cmd"`
	Env    map[string]string `json:"env"`
}

func newMockModules() modules {
	return modules{
		Parser:  &mockParser{},
		Builder: &mockBuilder{},
		Cache:   &mockCache{},
		Layer:   &mockLayer{},
		Image:   newMockImageStore(),
		Runtime: &mockRuntime{},
	}
}

func (m *mockParser) Parse(filePath string) ([]internal.Instruction, error) {
	if strings.TrimSpace(filePath) == "" {
		return nil, errors.New("empty Docksmithfile path")
	}
	return []internal.Instruction{
		{Raw: "FROM base"},
		{Raw: "COPY . /app"},
		{Raw: "RUN echo hello"},
	}, nil
}

func (m *mockBuilder) Build(instructions []internal.Instruction, tag string, context string, noCache bool) (internal.Image, error) {
	_ = context
	_ = noCache
	if len(instructions) == 0 {
		return internal.Image{}, errors.New("no instructions provided")
	}
	parts := strings.SplitN(tag, ":", 2)
	if len(parts) != 2 {
		return internal.Image{}, fmt.Errorf("invalid tag %q", tag)
	}
	defaultCmd := []string{"echo", "container running"}
	if parts[0] == "nocmd" {
		defaultCmd = nil
	}
	return internal.Image{
		Name:      parts[0],
		Tag:       parts[1],
		Digest:    "sha256:dummy",
		CreatedAt: time.Now(),
		Config: internal.ImageConfig{
			Cmd: defaultCmd,
			Env: map[string]string{"APP_ENV": "dev"},
		},
	}, nil
}

func (m *mockCache) Check(key string) (string, bool) {
	_ = key
	return "", false
}

func (m *mockCache) Store(key string, layerDigest string) error {
	_ = key
	_ = layerDigest
	return nil
}

func (m *mockLayer) CreateLayer(prevLayer string, instruction internal.Instruction, context string) (string, int64, error) {
	_ = prevLayer
	_ = instruction
	_ = context
	return "sha256:layer-dummy", 0, nil
}

func newMockImageStore() *mockImageStore {
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	return &mockImageStore{
		images: make(map[string]internal.Image),
		dbPath: filepath.Join(wd, ".docksmith", "mock-images.json"),
	}
}

func (m *mockImageStore) Save(image internal.Image) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadLocked(); err != nil {
		return err
	}
	key := imageKey(image.Name + ":" + image.Tag)
	m.images[key] = image
	return m.persistLocked()
}

func (m *mockImageStore) Load(nameTag string) (internal.Image, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadLocked(); err != nil {
		return internal.Image{}, err
	}
	img, ok := m.images[imageKey(nameTag)]
	if !ok {
		return internal.Image{}, fmt.Errorf("image %s not found", nameTag)
	}
	return img, nil
}

func (m *mockImageStore) List() ([]internal.Image, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadLocked(); err != nil {
		return nil, err
	}
	out := make([]internal.Image, 0, len(m.images))
	for _, img := range m.images {
		out = append(out, img)
	}
	sort.Slice(out, func(i, j int) bool {
		li := out[i].Name + ":" + out[i].Tag
		lj := out[j].Name + ":" + out[j].Tag
		return li < lj
	})
	return out, nil
}

func (m *mockImageStore) Remove(nameTag string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadLocked(); err != nil {
		return err
	}
	key := imageKey(nameTag)
	if _, ok := m.images[key]; !ok {
		return fmt.Errorf("image %s not found", nameTag)
	}
	delete(m.images, key)
	return m.persistLocked()
}

func (m *mockImageStore) loadLocked() error {
	if m.loaded {
		return nil
	}
	m.loaded = true
	data, err := os.ReadFile(m.dbPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("load image store: %w", err)
	}
	var records []imageRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return fmt.Errorf("decode image store: %w", err)
	}
	for _, rec := range records {
		img := internal.Image{
			Name:   rec.Name,
			Tag:    rec.Tag,
			Digest: rec.Digest,
			Config: internal.ImageConfig{Cmd: rec.Cmd, Env: rec.Env},
		}
		m.images[imageKey(rec.Name+":"+rec.Tag)] = img
	}
	return nil
}

func (m *mockImageStore) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(m.dbPath), 0o755); err != nil {
		return fmt.Errorf("prepare image store path: %w", err)
	}
	records := make([]imageRecord, 0, len(m.images))
	for _, img := range m.images {
		records = append(records, imageRecord{
			Name:   img.Name,
			Tag:    img.Tag,
			Digest: img.Digest,
			Cmd:    img.Config.Cmd,
			Env:    img.Config.Env,
		})
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Name+":"+records[i].Tag < records[j].Name+":"+records[j].Tag
	})
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("encode image store: %w", err)
	}
	if err := os.WriteFile(m.dbPath, data, 0o644); err != nil {
		return fmt.Errorf("persist image store: %w", err)
	}
	return nil
}

func (m *mockRuntime) Run(image internal.Image, cmd []string, env map[string]string) (int, error) {
	_ = image
	_ = cmd
	_ = env
	fmt.Println("container running")
	return 0, nil
}

func imageKey(nameTag string) string {
	return strings.TrimSpace(strings.ToLower(nameTag))
}
