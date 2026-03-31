package internal

import (
	"fmt"
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
	Cmd []string
	Env map[string]string
}

// Image represents a built or loaded image.
type Image struct {
	Name      string
	Tag       string
	Digest    string
	CreatedAt time.Time
	Config    ImageConfig
}

// Parser parses Docksmithfile instructions.
type Parser interface {
	Parse(filePath string) ([]Instruction, error)
}

// Builder builds images from parsed instructions.
type Builder interface {
	Build(instructions []Instruction, tag string, context string, noCache bool) (Image, error)
}

// Cache handles layer cache lookups and persistence.
type Cache interface {
	Check(key string) (layerDigest string, hit bool)
	Store(key string, layerDigest string) error
}

// Layer creates image layers and their digests.
type Layer interface {
	CreateLayer(prevLayer string, instruction Instruction, context string) (digest string, size int64, err error)
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
