# Docksmith Engine

A simplified Docker-like image build + container runtime system implemented in Go.

Docksmith parses a `Docksmithfile`, builds layered images with a deterministic cache, and runs containers using `chroot`-based isolation.

## ✅ Project Status — COMPLETE

| Module        | Owner     | Status |
|---------------|-----------|--------|
| CLI           | Gunadeep  | ✅ DONE |
| Parser        | Deepak    | ✅ DONE |
| Layer + Image | Chinmay   | ✅ DONE |
| Cache         | Vishnu    | ✅ DONE |
| Runtime       | All       | ✅ DONE (Linux / WSL2 only) |

## 🏗️ Architecture

```text
CLI → Parser → Builder → (Cache + Layer + Image)
                           ↓
                        Runtime
```

## ⚠️ Important Requirements

### ✅ Must use

- Linux **or** WSL2 (Ubuntu recommended)

### ❌ Do not use

- Windows CMD / PowerShell
- Docker (not required)

### Why?

The runtime uses Linux `chroot(2)` for isolation, which only works on Linux/WSL2.

## ✅ Prerequisites

- Go **1.22+** installed (verify with `go version`)
- Git installed (to clone the repo)
- Linux machine **or** WSL2 with an Ubuntu distro
- `sudo` access (required to run containers via `chroot`)

## 🚀 How to Run (Linux / WSL2)

### One rule (avoid “image not found”)

Always use the **same user** for both build and run.

- Recommended: use `sudo` for **both** build and run
- Reason: build artifacts are stored under `~/.docksmith/` for the running user
  - build without sudo → `~/.docksmith`
  - run with sudo → `/root/.docksmith`

### 1) Build Image (first build = cache miss)

```bash
sudo go run ./cmd build -t test:latest .
```

Expected behavior (first build):

- `COPY` → `CACHE MISS`
- `RUN` → `CACHE MISS`

### 2) Build Again (cache hit check)

```bash
sudo go run ./cmd build -t test:latest .
```

Expected behavior:

- `COPY` → `CACHE HIT`
- `RUN` → `CACHE HIT`

### 3) Run Container

```bash
sudo go run ./cmd run test:latest
```

Expected:

- Program output printed
- Exit code shown

### 4) List Images

```bash
go run ./cmd images
```

### 5) Remove Image

```bash
go run ./cmd rmi test:latest
```

## 🧪 Important Tests

### ✅ Cache Invalidation (Cascade Miss)

Change something in the build context:

```bash
echo "change" >> file.txt
sudo go run ./cmd build -t test:latest .
```

Expected:

- `COPY` → `MISS`
- `RUN` → `MISS` (cascade)

### 🚨 Isolation Test (Most Important)

1) Modify `Docksmithfile`:

```text
CMD ["sh","-c","echo hacked > /test.txt"]
```

2) Build + Run:

```bash
sudo go run ./cmd build -t test:latest .
sudo go run ./cmd run test:latest
```

3) Check the host machine:

```bash
ls /test.txt
```

✅ Expected:

`ls: cannot access '/test.txt': No such file or directory`

This confirms the container cannot write to the host filesystem.

## 🧠 Features

### CLI

- Commands: `build`, `run`, `images`, `rmi`

### Parser

- Supports: `FROM`, `COPY`, `RUN`, `WORKDIR`, `ENV`, `CMD`
- Preserves the raw instruction line for correct cache keys

### Layer System

- Only `COPY` and `RUN` create layers
- Layers stored in `~/.docksmith/layers/`
- Deterministic output (sorted entries, zero timestamps)

### Image System

- Images stored in `~/.docksmith/images/`
- Manifest includes config + ordered layer digests + metadata

### Cache System

- Cache stored in `~/.docksmith/cache/`
- Deterministic cache key includes:
  - previous layer digest
  - raw instruction string
  - `WORKDIR`
  - `ENV` (sorted)
  - `COPY` file hashes
- Supports `--no-cache`

### Runtime (Linux / WSL2)

- Extracts layers into a temporary rootfs
- Uses `chroot()` for isolation
- Applies `ENV` + `WORKDIR`
- Executes the configured command safely

## ⚠️ Strict Rules Followed

- Only `COPY` & `RUN` create layers
- Deterministic builds (same input → same digest)
- Full cache key (no shortcuts)
- No network usage during build
- Runtime isolation enforced via `chroot`

## 🎬 Demo Flow (Evaluator Friendly)

1. Build → `CACHE MISS`
2. Build again → `CACHE HIT`
3. Run → output shown
4. Isolation test → proves no `/test.txt` on host
5. `images` → lists images
6. `rmi` → removes image

## 🎓 Conclusion

Docksmith Engine is a fully functional, deterministic, and isolated mini container engine built from scratch in Go.
