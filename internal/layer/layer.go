package layer

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"docksmith-engine/internal"
)

const (
	digestPrefix = "sha256:"
)

// Service implements internal.Layer with deterministic delta tar layers.
type Service struct {
	root string
}

// Option configures Service construction.
type Option func(*Service)

// WithDataRoot sets the Docksmith data directory (default: ~/.docksmith).
func WithDataRoot(root string) Option {
	return func(s *Service) {
		if strings.TrimSpace(root) != "" {
			s.root = root
		}
	}
}

// New creates a layer service.
func New(opts ...Option) *Service {
	h, err := os.UserHomeDir()
	if err != nil {
		h = "."
	}
	s := &Service{root: filepath.Join(h, ".docksmith")}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Service) layersDir() string {
	return filepath.Join(s.root, "layers")
}

// CreateLayer builds a delta layer for COPY or RUN only.
func (s *Service) CreateLayer(prevLayerDigest string, instruction internal.Instruction, context string) (string, int64, error) {
	op := strings.ToUpper(strings.TrimSpace(instruction.Op))
	if op != "COPY" && op != "RUN" {
		return "", 0, fmt.Errorf("layer: CreateLayer supports COPY and RUN only, got %q", instruction.Op)
	}
	if err := validateDigest(prevLayerDigest, true); err != nil {
		return "", 0, err
	}
	ctxAbs, err := filepath.Abs(context)
	if err != nil {
		return "", 0, fmt.Errorf("layer: context path: %w", err)
	}
	if st, err := os.Stat(ctxAbs); err != nil || !st.IsDir() {
		if err != nil {
			return "", 0, fmt.Errorf("layer: context %q: %w", context, err)
		}
		return "", 0, fmt.Errorf("layer: context %q is not a directory", context)
	}

	workDir, err := os.MkdirTemp("", "docksmith-layer-*")
	if err != nil {
		return "", 0, fmt.Errorf("layer: temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	chain, err := s.chainBottomToTop(prevLayerDigest)
	if err != nil {
		return "", 0, err
	}
	for _, d := range chain {
		if err := s.extractLayerInto(workDir, d); err != nil {
			return "", 0, err
		}
	}

	before, err := snapshotTree(workDir)
	if err != nil {
		return "", 0, err
	}

	switch op {
	case "COPY":
		if len(instruction.Args) < 2 {
			return "", 0, fmt.Errorf("layer: COPY requires source and destination")
		}
		if err := applyCopy(workDir, ctxAbs, instruction.Args); err != nil {
			return "", 0, err
		}
	case "RUN":
		if err := applyRunMock(workDir, instruction.Args); err != nil {
			return "", 0, err
		}
	}

	after, err := snapshotTree(workDir)
	if err != nil {
		return "", 0, err
	}

	paths := deltaPaths(before, after)
	tarData, err := buildDeterministicTar(workDir, paths)
	if err != nil {
		return "", 0, err
	}

	sum := sha256.Sum256(tarData)
	digest := digestPrefix + hex.EncodeToString(sum[:])

	if err := os.MkdirAll(s.layersDir(), 0o755); err != nil {
		return "", 0, fmt.Errorf("layer: mkdir layers: %w", err)
	}
	tarPath := s.layerTarPath(digest)
	if st, err := os.Stat(tarPath); err == nil {
		// Immutable: identical tar already stored; never rewrite.
		return digest, st.Size(), nil
	} else if !os.IsNotExist(err) {
		return "", 0, fmt.Errorf("layer: stat layer: %w", err)
	}
	if err := os.WriteFile(tarPath, tarData, 0o644); err != nil {
		return "", 0, fmt.Errorf("layer: write tar: %w", err)
	}
	if err := s.writeParentFile(digest, prevLayerDigest); err != nil {
		return "", 0, err
	}

	return digest, int64(len(tarData)), nil
}

func validateDigest(d string, allowEmpty bool) error {
	d = strings.TrimSpace(d)
	if d == "" {
		if allowEmpty {
			return nil
		}
		return fmt.Errorf("layer: empty digest")
	}
	if !strings.HasPrefix(d, digestPrefix) {
		return fmt.Errorf("layer: digest must start with %q", digestPrefix)
	}
	hexPart := strings.TrimPrefix(d, digestPrefix)
	if len(hexPart) != 64 {
		return fmt.Errorf("layer: invalid digest length")
	}
	for _, c := range hexPart {
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'f' {
			continue
		}
		return fmt.Errorf("layer: digest contains non-hex character")
	}
	return nil
}

func digestHex(digest string) string {
	return strings.TrimPrefix(strings.TrimSpace(digest), digestPrefix)
}

func (s *Service) layerTarPath(digest string) string {
	return filepath.Join(s.layersDir(), fmt.Sprintf("sha256_%s.tar", digestHex(digest)))
}

func (s *Service) layerParentPath(digest string) string {
	return filepath.Join(s.layersDir(), fmt.Sprintf("sha256_%s.parent", digestHex(digest)))
}

type parentFile struct {
	Parent string `json:"parent"`
}

func (s *Service) readParent(digest string) (string, error) {
	p := s.layerParentPath(digest)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("layer: missing parent metadata for %s", digest)
		}
		return "", err
	}
	var pf parentFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return "", fmt.Errorf("layer: parent json: %w", err)
	}
	if pf.Parent != "" {
		if err := validateDigest(pf.Parent, false); err != nil {
			return "", err
		}
	}
	return pf.Parent, nil
}

func (s *Service) writeParentFile(digest, parentDigest string) error {
	pf := parentFile{Parent: strings.TrimSpace(parentDigest)}
	data, err := json.Marshal(pf)
	if err != nil {
		return err
	}
	return os.WriteFile(s.layerParentPath(digest), data, 0o644)
}

// chainBottomToTop returns [oldest ... newest] layer digests ending at topDigest.
func (s *Service) chainBottomToTop(topDigest string) ([]string, error) {
	if strings.TrimSpace(topDigest) == "" {
		return nil, nil
	}
	if err := validateDigest(topDigest, false); err != nil {
		return nil, err
	}
	var rev []string
	for cur := topDigest; cur != ""; {
		rev = append(rev, cur)
		p, err := s.readParent(cur)
		if err != nil {
			return nil, err
		}
		cur = p
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev, nil
}

func (s *Service) extractLayerInto(root string, digest string) error {
	tarPath := s.layerTarPath(digest)
	f, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("layer: open %s: %w", digest, err)
	}
	defer f.Close()
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("layer: read tar: %w", err)
		}
		if err := safeExtractTarEntry(root, tr, hdr); err != nil {
			return err
		}
	}
	return nil
}

func safeExtractTarEntry(root string, tr io.Reader, hdr *tar.Header) error {
	clean := path.Clean(hdr.Name)
	if clean == "." || strings.HasPrefix(clean, "..") {
		return fmt.Errorf("layer: invalid tar path %q", hdr.Name)
	}
	target := filepath.Join(root, filepath.FromSlash(clean))
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(targetAbs+string(os.PathSeparator), rootAbs+string(os.PathSeparator)) && targetAbs != rootAbs {
		return fmt.Errorf("layer: path escapes root: %q", hdr.Name)
	}
	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(targetAbs, fs.FileMode(hdr.Mode)&0o777)
	case tar.TypeReg, tar.TypeRegA:
		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(targetAbs, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(hdr.Mode)&0o777)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	case tar.TypeSymlink:
		return fmt.Errorf("layer: symlinks not supported in extract")
	default:
		return nil
	}
}

type snap map[string][32]byte // relative slash path -> sha256

func snapshotTree(root string) (snap, error) {
	m := make(snap)
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if shouldIgnoreName(filepath.Base(p)) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		key := filepath.ToSlash(rel)
		if d.IsDir() {
			m[key] = [32]byte{}
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		m[key] = sha256.Sum256(data)
		return nil
	})
	return m, err
}

func shouldIgnoreName(name string) bool {
	// Exclude known nondeterministic/system metadata.
	if name == ".docksmith" {
		return true
	}
	// The build definition should not be copied into image layers.
	if name == "Docksmithfile" {
		return true
	}
	if name == ".DS_Store" || name == "Thumbs.db" || name == "__MACOSX" {
		return true
	}
	if strings.HasPrefix(name, "._") {
		return true
	}
	return false
}

func deltaPaths(before, after snap) []string {
	var out []string
	for p, h := range after {
		prev, ok := before[p]
		if !ok || prev != h {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func buildDeterministicTar(root string, relPaths []string) ([]byte, error) {
	epoch := time.Unix(0, 0).UTC()

	if len(relPaths) == 0 {
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		if err := tw.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	dirsNeeded := make(map[string]struct{})
	for _, rp := range relPaths {
		dir := path.Dir(filepath.ToSlash(rp))
		for dir != "." && dir != "/" {
			dirsNeeded[dir] = struct{}{}
			dir = path.Dir(dir)
		}
	}

	entryNames := make(map[string]struct{})
	for d := range dirsNeeded {
		entryNames[d+"/"] = struct{}{}
	}
	for _, rp := range relPaths {
		rp = filepath.ToSlash(rp)
		diskPath := filepath.Join(root, filepath.FromSlash(rp))
		if shouldIgnoreName(filepath.Base(diskPath)) {
			continue
		}
		st, err := os.Stat(diskPath)
		if err != nil {
			return nil, fmt.Errorf("layer: stat %s: %w", rp, err)
		}
		if st.IsDir() {
			entryNames[rp+"/"] = struct{}{}
			continue
		}
		entryNames[rp] = struct{}{}
	}

	all := make([]string, 0, len(entryNames))
	for n := range entryNames {
		all = append(all, n)
	}
	sort.Strings(all)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, name := range all {
		isDir := strings.HasSuffix(name, "/")
		p := strings.TrimSuffix(name, "/")
		diskPath := filepath.Join(root, filepath.FromSlash(p))
		var hdr tar.Header
		if isDir {
			hdr.Name = p + "/"
			hdr.Mode = 0o755
			hdr.Typeflag = tar.TypeDir
		} else {
			st, err := os.Stat(diskPath)
			if err != nil {
				return nil, fmt.Errorf("layer: missing file %s: %w", p, err)
			}
			if st.IsDir() {
				hdr.Name = p + "/"
				hdr.Mode = 0o755
				hdr.Typeflag = tar.TypeDir
			} else {
				hdr.Name = p
				hdr.Mode = 0o644
				hdr.Size = st.Size()
				hdr.Typeflag = tar.TypeReg
			}
		}
		hdr.Uid = 0
		hdr.Gid = 0
		hdr.Uname = ""
		hdr.Gname = ""
		hdr.ModTime = epoch
		hdr.AccessTime = epoch
		hdr.ChangeTime = epoch
		// Use PAX so access/change times are encoded deterministically.
		hdr.Format = tar.FormatPAX
		if err := tw.WriteHeader(&hdr); err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeReg {
			f, err := os.Open(diskPath)
			if err != nil {
				return nil, err
			}
			_, err = io.Copy(tw, f)
			f.Close()
			if err != nil {
				return nil, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
