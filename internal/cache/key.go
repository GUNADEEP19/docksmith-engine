package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// KeyHash returns a deterministic hex-encoded SHA-256 of all cache key components.
// prevDigest is the previous layer digest (empty after FROM scratch with no layers yet).
// raw is the exact parser Raw line for the instruction.
// workdir is the current WORKDIR in the container.
// env is the full ENV map; keys are sorted lexicographically for the key material.
// For COPY, copyArgs must be the original Args from the parser; ctxAbs is the absolute build context path.
// For RUN, copyArgs should be nil and ctxAbs may be empty.
func KeyHash(prevDigest, raw, workdir string, env map[string]string, op string, copyArgs []string, ctxAbs string) (string, error) {
	op = strings.ToUpper(strings.TrimSpace(op))
	var buf bytes.Buffer
	buf.WriteString(prevDigest)
	buf.WriteByte(0)
	buf.WriteString(raw)
	buf.WriteByte(0)
	buf.WriteString(workdir)
	buf.WriteByte(0)
	buf.WriteString(encodeEnvSorted(env))
	buf.WriteByte(0)
	if op == "COPY" {
		if ctxAbs == "" {
			return "", fmt.Errorf("cache: COPY requires absolute context path")
		}
		copyBlock, err := copySourcesHashBlock(ctxAbs, copyArgs)
		if err != nil {
			return "", err
		}
		buf.WriteString(copyBlock)
	} else if op != "RUN" {
		return "", fmt.Errorf("cache: unsupported op %q", op)
	}
	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:]), nil
}

func encodeEnvSorted(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(env[k])
	}
	return b.String()
}
