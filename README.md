# Docksmith Engine

A simplified Docker-like image build + container runtime system implemented in Go.

Docksmith parses a `Docksmithfile`, builds layered images with a deterministic cache, and runs containers using `chroot`-based isolation.

## 🏗️ Architecture

```text
CLI → Parser → Builder → (Cache + Layer + Image)
                           ↓
                        Runtime
```

## ⚠️ Requirements (Read This)

- **Linux or WSL2 is required** (the runtime uses Linux `chroot(2)`). macOS/Windows are not supported for `build`/`run`.
- **Offline only**: no downloads during build or run (don’t use `apt-get`, `curl`, etc. in `RUN`).
- **Only `COPY` and `RUN` create layers**. Other instructions only update build state.
- Cache reporting is printed only for `COPY` and `RUN` steps: `Step i/n : ... [CACHE HIT|MISS] X.XXs`.

## ✅ Prerequisites

- Go **1.22+**
- Linux machine **or** WSL2 (Ubuntu recommended)
- `sudo` access (needed for `chroot`)

## 🎬 Demo Script (Copy/Paste)

This repo includes a required offline sample app in `sample-app/` that uses all six instructions: `FROM`, `WORKDIR`, `COPY`, `ENV`, `RUN`, `CMD`.

### Rule: use the same user for build + run

Use `sudo` for both build and run so everything is stored under the same `~/.docksmith/` location (for root).

### 0) One-time base image import (offline `FROM base:latest`)

```bash
sudo go run ./cmd base base:latest
```

### 1) Cold build (expect cache MISS)

```bash
sudo go run ./cmd build -t sample:latest ./sample-app
```

Expected behavior:

- `COPY` → `[CACHE MISS]`
- `RUN` → `[CACHE MISS]`

### 2) Warm build (expect cache HIT)

```bash
sudo go run ./cmd build -t sample:latest ./sample-app
```

Expected behavior:

- `COPY` → `[CACHE HIT]`
- `RUN` → `[CACHE HIT]`

### 3) Cache invalidation + cascade (COPY MISS → RUN MISS)

Change a copied file, then rebuild:

```bash
echo "# demo change" >> sample-app/app/main.sh
sudo go run ./cmd build -t sample:latest ./sample-app
```

Expected behavior:

- `COPY` → `[CACHE MISS]`
- `RUN` → `[CACHE MISS]` (cascade)

### 4) Run (visible output)

```bash
sudo go run ./cmd run sample:latest
```

### 5) ENV override demo

```bash
sudo go run ./cmd run -e MESSAGE="Hi Professor" sample:latest
```

### 6) Isolation proof (container write must NOT appear on host)

Run a command that writes `/test.txt` **inside** the container rootfs:

```bash
sudo go run ./cmd run sample:latest sh -c 'echo hacked > /test.txt'
```

Now check the host filesystem:

```bash
ls /test.txt
```

Expected:

`ls: cannot access '/test.txt': No such file or directory`

### 7) List images + remove image

```bash
go run ./cmd images
go run ./cmd rmi sample:latest
```

### (Optional) Force rebuild (skip cache)

```bash
sudo go run ./cmd build -t sample:latest --no-cache ./sample-app
```

## 🚀 General Usage (Linux / WSL2)

### Build

```bash
sudo go run ./cmd build -t test:latest .
```

### Run

```bash
sudo go run ./cmd run test:latest
```

### Run with ENV override

```bash
sudo go run ./cmd run -e KEY=value test:latest
```

### List / Remove

```bash
go run ./cmd images
go run ./cmd rmi test:latest
```

## 🧪 Tests

```bash
go test ./... -v
```
