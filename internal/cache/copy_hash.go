package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// copySourcesHashBlock mirrors layer COPY source resolution and hashes file contents.
// Paths are relative to the build context, sorted lexicographically (POSIX slashes).
func copySourcesHashBlock(ctxAbs string, args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("COPY requires source and destination")
	}
	// Include the exact COPY args (including destination) so semantic changes
	// to the instruction cannot reuse a previous layer.
	var header strings.Builder
	for i, a := range args {
		if i > 0 {
			header.WriteByte('\n')
		}
		header.WriteString(strings.TrimSpace(a))
	}
	header.WriteString("\n--\n")

	sources := args[:len(args)-1]
	seen := make(map[string]struct{})
	for _, src := range sources {
		if err := collectCopyPaths(ctxAbs, strings.TrimSpace(src), seen); err != nil {
			return "", err
		}
	}
	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var b strings.Builder
	b.WriteString(header.String())
	for _, rel := range paths {
		disk := filepath.Join(ctxAbs, filepath.FromSlash(rel))
		h, err := hashFileContent(disk)
		if err != nil {
			return "", err
		}
		b.WriteString(rel)
		b.WriteByte('\t')
		b.WriteString(h)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func hashFileContent(diskPath string) (string, error) {
	f, err := os.Open(diskPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func collectCopyPaths(ctxRoot, srcArg string, seen map[string]struct{}) error {
	srcPath := filepath.Join(ctxRoot, srcArg)
	st, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	srcIsDot := srcArg == "." || srcArg == "./."
	if srcIsDot {
		return walkContextContents(ctxRoot, filepath.Join(ctxRoot, "."), seen)
	}
	if st.IsDir() {
		return walkDirTree(srcPath, ctxRoot, seen)
	}
	rel, err := filepath.Rel(ctxRoot, srcPath)
	if err != nil {
		return err
	}
	addSeen(seen, rel)
	return nil
}

func walkContextContents(ctxRoot, dir string, seen map[string]struct{}) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		if shouldIgnoreName(name) {
			continue
		}
		full := filepath.Join(dir, name)
		st, err := os.Stat(full)
		if err != nil {
			return err
		}
		if st.IsDir() {
			if err := walkDirTree(full, ctxRoot, seen); err != nil {
				return err
			}
		} else {
			rel, err := filepath.Rel(ctxRoot, full)
			if err != nil {
				return err
			}
			addSeen(seen, rel)
		}
	}
	return nil
}

func walkDirTree(srcDir, ctxRoot string, seen map[string]struct{}) error {
	return filepath.WalkDir(srcDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if shouldIgnoreName(filepath.Base(p)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(ctxRoot, p)
		if err != nil {
			return err
		}
		addSeen(seen, rel)
		return nil
	})
}

func addSeen(seen map[string]struct{}, rel string) {
	if rel == "." {
		return
	}
	key := filepath.ToSlash(rel)
	seen[key] = struct{}{}
}

func shouldIgnoreName(name string) bool {
	if name == ".docksmith" {
		return true
	}
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
