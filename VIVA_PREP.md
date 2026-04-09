# Docksmith Viva Prep (Professor/TA)

This handout is designed to be memorized and presented. It matches the project constraints:
- No network during build/run (offline after base setup)
- No Docker/runc/containerd (OS primitives only)
- Immutable content-addressed layers
- RUN during build executes inside the image filesystem using the same isolation as runtime
- Deterministic builds (same input → same output digest)

---

## 1) What is Docksmith?

Docksmith is a simplified, Docker-inspired container system built to understand the core ideas behind containers.

It has three core systems:
1. **Image build system** (parses a Docksmithfile and produces a layered image)
2. **Deterministic cache system** (reuses layers when inputs are unchanged)
3. **Container runtime** (runs the built image with OS-level filesystem isolation)

One-line summary you can say in viva:

> Docksmith is a deterministic, layer-based container engine with a correct cache and a chroot-isolated runtime built using OS primitives.

---

## 2) Architecture (say this while showing the diagram)

Flow:

`CLI → Parser → Builder → (Cache + Layer + ImageStore) → Runtime`

What each piece does:
- **CLI**: reads args, wires modules, prints progress/logs; no business logic.
- **Parser**: reads Docksmithfile, validates instructions, preserves raw text.
- **Builder**: walks instructions, maintains state (WORKDIR/ENV/CMD), creates layers on COPY/RUN.
- **Cache**: maps deterministic cache-key → layer digest.
- **Layer**: creates deterministic tar layer deltas and digests.
- **ImageStore**: saves/loads a JSON manifest (config + ordered layers + metadata).
- **Runtime**: extracts layers into a temp rootfs and executes the command inside chroot.

---

## 3) Core Concepts (high scoring viva topics)

### A) Layered filesystem
- Only **COPY** and **RUN** create filesystem changes → only they produce layers.
- A layer is a **tar archive delta** (not a full snapshot).
- Layers are stored content-addressed (SHA-256 digest):
  - Same layer content → same digest → reusable across builds.

### B) Deterministic builds
Definition:
- **Same Docksmithfile + same context files + same state** must produce **exactly identical digests**.

How we ensure it:
- Tar entries are written in a consistent sorted order.
- File timestamps are zeroed.
- Manifest hashing is stable.

Why it matters:
- If build output varies, cache keys won’t match → cache never hits.

### C) Cache key + cascade invalidation (most important logic)
Cache key includes:
- Previous layer digest (or base image digest seed)
- Raw instruction text (exact formatting matters)
- WORKDIR
- ENV (sorted serialization)
- For COPY: file content hashes (sorted)

Behavior:
- Same inputs → **CACHE HIT** (reuse digest)
- Any change → **CACHE MISS**
- One MISS cascades → all following COPY/RUN steps also MISS (because prevDigest changed)

### D) Process isolation (runtime)
Mechanism:
- Docksmith uses **chroot()** on Linux.

What it guarantees:
- The process sees the container root filesystem as `/`.
- A file written inside the container must NOT appear on host.

Most important rule to say:

> We use the same isolation mechanism for build-time RUN and runtime execution.

### E) Image system
- Image is a **JSON manifest** stored locally.
- Contains:
  - config: ENV, WORKDIR, CMD
  - ordered list of layer digests
  - digest (hash of canonical manifest content)

### F) Content addressing
Everything important is identified by SHA-256:
- Layer tar digest
- Cache key digest
- Image manifest digest

---

## 4) Team Contributions (what each person should say)

### Gunadeep — CLI + orchestration
What you did:
- Implemented command parsing and routing: build/run/images/rmi/base
- Ensured correct user-facing progress logging format
- Kept CLI free of business logic; all real logic is in modules

Theory points:
- Separation of concerns
- Interface-driven design (DI/wiring)
- Stable UX/logging for evaluation

Viva question: “Why doesn’t CLI contain the build logic?”
- Answer: So modules are testable and reusable; CLI remains a thin adapter.

### Deepak — Parser
What you did:
- Parsed Docksmithfile instructions and validated syntax
- Preserved the raw instruction string exactly

Theory points:
- Parsing + validation
- Error reporting with line numbers
- Raw instruction text is critical for cache correctness

Viva question: “Why preserve raw text instead of reconstructing?”
- Answer: Cache key must include the exact instruction text; reconstruction could change whitespace and break determinism.

### Chinmay — Layers + images
What you did:
- Created deterministic tar layer deltas for COPY/RUN
- Maintained image manifest format and digesting

Theory points:
- Filesystem layering
- Content-addressed storage
- Deterministic archive creation

Viva question: “Why do only COPY and RUN create layers?”
- Answer: Only those modify filesystem state; ENV/WORKDIR/CMD are metadata/state.

### Vishnu — Cache
What you did:
- Implemented deterministic cache key generation and storage
- Ensured cache is correct (not just fast)

Theory points:
- Hash-based caching
- Dependency tracking via prevDigest
- Cascade invalidation

Viva question: “What happens if ENV changes?”
- Answer: Cache key changes for subsequent steps, causing cache misses downstream.

### All — Runtime + isolation parity
What you did:
- Extracted layers into a temp rootfs
- Applied ENV + WORKDIR
- Executed command inside chroot
- Ensured build-time RUN uses the same isolation primitive

---

## 5) Demo Script + What It Proves

These commands are in the README. Narration is what you should say.

### Step 0: One-time base import
Command:
- `sudo go run ./cmd base base:latest`

Say:
- “We import a local base reference once. After this, everything runs offline.”

### Step 1: Cold build (MISS)
Command:
- `sudo go run ./cmd build -t sample:latest ./sample-app`

Say:
- “First build must miss cache; COPY and RUN layers are created.”

### Step 2: Warm build (HIT)
Command:
- `sudo go run ./cmd build -t sample:latest ./sample-app`

Say:
- “Second build should be cache hit; it reuses identical layer digests.”

### Step 3: Invalidation + cascade
Commands:
- `date > sample-app/.demo-change.txt`
- `sudo go run ./cmd build -t sample:latest ./sample-app`

Say:
- “Changing a copied input forces COPY to miss and because prevDigest changes, RUN also misses (cascade).”

### Step 4: Run (visible output)
Command:
- `sudo go run ./cmd run sample:latest`

Say:
- “This prints visible output from inside the container filesystem.”

### Step 5: ENV override
Command:
- `sudo go run ./cmd run -e MESSAGE="Hi Professor" sample:latest`

Say:
- “Runtime -e overrides the image ENV value.”

### Step 6: Isolation proof (pass/fail)
Commands:
- `sudo go run ./cmd run sample:latest sh -c 'echo hacked > /test.txt'`
- `ls /test.txt`

Say:
- “The write happens inside the container root. On the host, /test.txt must not exist.”

---

## 6) Likely Viva Questions (and bulletproof answers)

### Q1: “What makes this different from Docker?”
- No daemon, no networking stack, no namespaces/cgroups; simplified pipeline to teach core concepts.

### Q2: “Why is determinism necessary?”
- Without determinism, hashes change between builds → cache cannot hit reliably.

### Q3: “What exactly goes into the cache key?”
- prevDigest + raw instruction + WORKDIR + sorted ENV + (for COPY) file content hashes.

### Q4: “Why does one cache miss cascade?”
- Because the next key includes prevDigest; once prevDigest changes, all downstream keys change.

### Q5: “How do you prove RUN executes inside the image FS?”
- RUN produces a filesystem artifact inside the rootfs (e.g., /app/build.txt) and isolation is the same mechanism used for runtime.

### Q6: “How do you prove isolation?”
- Write `/test.txt` inside the container and demonstrate it does not exist on host (`ls /test.txt` fails).

### Q7: “Where is the image stored?”
- As a JSON manifest under the local store (see README for the `~/.docksmith/` layout).

### Q8: “What about the ‘base image’ concern?”
If asked aggressively:
- “In this project, base images are local references required by the spec. The runtime/build pipelines provision the minimal userland needed for `RUN`/`CMD` deterministically and offline. We do not download anything during build/run, and we do not call Docker/runc.”

---

## 7) Mandatory constraints (say these explicitly)

- Offline after base setup: no network requests in build/run.
- No existing container runtimes.
- Immutable layers: once written, never modified.
- Verified isolation: container writes never appear on host.
- Reproducible builds: same input → identical digests.
- Manifest timestamp: created is preserved across full cache-hit rebuilds.
