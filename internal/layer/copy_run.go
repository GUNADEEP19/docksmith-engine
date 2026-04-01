package layer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

func applyRunMock(root string, args []string) error {
	h := sha256.Sum256([]byte(strings.Join(args, "\x00")))
	line := hex.EncodeToString(h[:]) + "\n"
	p := filepath.Join(root, ".docksmith-mock-run")
	return os.WriteFile(p, []byte(line), 0o644)
}

func applyCopy(root, ctx string, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("COPY requires source and destination")
	}
	destArg := args[len(args)-1]
	sources := args[:len(args)-1]
	destArg = strings.TrimSpace(destArg)
	destRel := strings.TrimPrefix(path.Clean("/"+strings.TrimPrefix(destArg, "/")), "/")
	destAbs := filepath.Join(root, filepath.FromSlash(destRel))

	multiSrc := len(sources) > 1
	// Treat destination as directory when:
	// - multiple sources
	// - destination ends with '/'
	// - destination does not exist yet
	stat, err := os.Stat(destAbs)
	destMissing := err != nil && os.IsNotExist(err)
	destIsDir := multiSrc || strings.HasSuffix(destArg, "/") || strings.HasSuffix(destArg, string(os.PathSeparator)) || destMissing
	if destIsDir {
		if err == nil && stat != nil && !stat.IsDir() {
			return fmt.Errorf("destination %q exists and is not a directory", destArg)
		}
		if mkErr := os.MkdirAll(destAbs, 0o755); mkErr != nil {
			return mkErr
		}
	}

	for _, src := range sources {
		src = strings.TrimSpace(src)
		srcPath := filepath.Join(ctx, src)
		if err := copyOneSource(ctx, srcPath, src, destAbs, destIsDir); err != nil {
			return fmt.Errorf("layer: COPY %q: %w", src, err)
		}
	}
	return nil
}

func copyOneSource(ctxRoot, srcPath, srcArg, destAbs string, destIsDirHint bool) error {
	st, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	// Destination must be treated as a directory when:
	// - source is a directory
	// - source is '.' (copy contents)
	// - applyCopy already determined destination should be directory
	srcIsDot := srcArg == "." || srcArg == "./."
	destMustBeDir := destIsDirHint || st.IsDir() || srcIsDot
	if destMustBeDir {
		if dstSt, stErr := os.Stat(destAbs); stErr == nil {
			if !dstSt.IsDir() {
				return fmt.Errorf("destination exists and is not a directory")
			}
		} else if os.IsNotExist(stErr) {
			if mkErr := os.MkdirAll(destAbs, 0o755); mkErr != nil {
				return mkErr
			}
		} else if stErr != nil {
			return stErr
		}
	}

	if srcIsDot {
		return copyDirContents(ctxRoot, destAbs)
	}

	if st.IsDir() {
		var target string
		if destMustBeDir {
			target = filepath.Join(destAbs, filepath.Base(srcPath))
		} else {
			target = destAbs
		}
		return copyDirTree(srcPath, target)
	}

	// regular file
	if destMustBeDir {
		if err := os.MkdirAll(destAbs, 0o755); err != nil {
			return err
		}
		return copyFile(srcPath, filepath.Join(destAbs, filepath.Base(srcPath)))
	}
	if err := os.MkdirAll(filepath.Dir(destAbs), 0o755); err != nil {
		return err
	}
	return copyFile(srcPath, destAbs)
}

func copyDirContents(srcDir, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(srcDir)
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
		from := filepath.Join(srcDir, name)
		to := filepath.Join(dstDir, name)
		st, err := os.Stat(from)
		if err != nil {
			return err
		}
		if st.IsDir() {
			if err := copyDirTree(from, to); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
				return err
			}
			if err := copyFile(from, to); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyDirTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if shouldIgnoreName(filepath.Base(p)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(p, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
