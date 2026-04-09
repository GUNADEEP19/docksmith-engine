package internal

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Instruction is the parsed representation of a Docksmithfile instruction.
type Instruction struct {
	Raw  string
	Op   string
	Args []string
}

func (i Instruction) String() string {
	if strings.TrimSpace(i.Raw) != "" {
		return i.Raw
	}
	if i.Op == "" {
		return ""
	}
	if len(i.Args) == 0 {
		return i.Op
	}
	return fmt.Sprintf("%s %s", i.Op, strings.Join(i.Args, " "))
}

// ImageConfig stores image runtime metadata used by the run orchestration path.
type ImageConfig struct {
	Cmd        []string
	Env        map[string]string
	WorkingDir string
}

// ImageLayer records one layer in an image manifest (COPY/RUN steps only).
type ImageLayer struct {
	Digest    string
	Size      int64
	CreatedBy string
}

// Image represents a built or loaded image.
type Image struct {
	Name      string
	Tag       string
	Digest    string
	CreatedAt time.Time
	Config    ImageConfig
	Layers    []ImageLayer
}

// Parser parses Docksmithfile instructions.
type Parser interface {
	Parse(filePath string) ([]Instruction, error)
}

// Builder builds images from parsed instructions.
type Builder interface {
	Build(instructions []Instruction, tag string, context string, noCache bool) (Image, error)
}

// BuildStep is emitted by builders that support progress callbacks.
// CacheStatus is only set for COPY/RUN; otherwise it is empty.
type BuildStep struct {
	Index           int
	Total           int
	Instruction     string
	CacheStatus     string
	DurationSeconds float64
}

// Cache handles layer cache lookups and persistence.
type Cache interface {
	Check(key string) (layerDigest string, hit bool)
	Store(key string, layerDigest string) error
}

// Layer creates image layers and their digests.
type Layer interface {
	CreateLayer(prevLayer string, instruction Instruction, context string, workdir string, env map[string]string) (digest string, size int64, err error)
}

// ImageStore persists and retrieves image metadata.
type ImageStore interface {
	Save(image Image) error
	Load(nameTag string) (Image, error)
	List() ([]Image, error)
	Remove(nameTag string) error
}

// Runtime runs a container from an image.
type Runtime interface {
	Run(image Image, cmd []string, env map[string]string) (exitCode int, err error)
}

// ReadFile is a helper function to read file contents.
// Used by parser and other modules that need to read files.
func ReadFile(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
