package image

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"docksmith-engine/internal"
)

const digestPrefix = "sha256:"

type manifestConfig struct {
	Env        []string `json:"Env"`
	Cmd        []string `json:"Cmd"`
	WorkingDir string   `json:"WorkingDir"`
}

type manifestLayer struct {
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	CreatedBy string `json:"createdBy"`
}

type manifest struct {
	Name    string          `json:"name"`
	Tag     string          `json:"tag"`
	Digest  string          `json:"digest"`
	Created string          `json:"created"`
	Config  manifestConfig  `json:"config"`
	Layers  []manifestLayer `json:"layers"`
}

func envMapToSlice(m map[string]string) []string {
	if len(m) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, fmt.Sprintf("%s=%s", k, m[k]))
	}
	return out
}

func toManifest(img internal.Image) manifest {
	created := ""
	if !img.CreatedAt.IsZero() {
		created = img.CreatedAt.UTC().Format(time.RFC3339)
	}
	ml := make([]manifestLayer, 0, len(img.Layers))
	for _, L := range img.Layers {
		ml = append(ml, manifestLayer{
			Digest:    L.Digest,
			Size:      L.Size,
			CreatedBy: L.CreatedBy,
		})
	}
	cmd := img.Config.Cmd
	if cmd == nil {
		cmd = []string{}
	}
	return manifest{
		Name:    img.Name,
		Tag:     img.Tag,
		Digest:  "",
		Created: created,
		Config: manifestConfig{
			Env:        envMapToSlice(img.Config.Env),
			Cmd:        append([]string(nil), cmd...),
			WorkingDir: img.Config.WorkingDir,
		},
		Layers: ml,
	}
}

// serializeManifest returns canonical JSON bytes (indented) and the manifest digest.
func serializeManifest(img internal.Image) (finalJSON []byte, digest string, err error) {
	m := toManifest(img)
	m.Digest = ""
	// Do not include Created in the digest input; it is metadata.
	m.Created = ""
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(raw)
	digest = digestPrefix + hex.EncodeToString(sum[:])
	m.Digest = digest
	// Emit Created in the stored JSON, but keep digest stable.
	m.Created = toManifest(img).Created
	finalJSON, err = json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, "", err
	}
	return finalJSON, digest, nil
}

// ImageManifestDigest returns the sha256 digest for the image manifest (digest field empty in hash input).
func ImageManifestDigest(img internal.Image) (string, error) {
	_, d, err := serializeManifest(img)
	return d, err
}
