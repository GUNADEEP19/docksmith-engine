# Docksmith Engine

A simplified Docker-like build and runtime system implemented in Go.

---

## ✅ Project Status — COMPLETE

All modules are fully implemented and verified:

| Module        | Owner    | Status                    |
|---------------|----------|---------------------------|
| CLI           | Gunadeep | ✅ DONE                    |
| Parser        | Deepak   | ✅ DONE                    |
| Layer + Image | Chinmay  | ✅ DONE                    |
| Cache         | Vishnu   | ✅ DONE                    |
| Runtime       | All      | ✅ DONE (Linux/WSL2 only) |

---

## 🏗️ Architecture

```
CLI → Parser → Builder → (Cache + Layer + Image)
                              ↓
                           Runtime
```

---

## 🚀 Features Implemented

### 🔹 CLI
- `build`, `run`, `images`, `rmi`
- Argument parsing and validation
- Strict logging format

### 🔹 Parser
- Supports: `FROM`, `COPY`, `RUN`, `WORKDIR`, `ENV`, `CMD`
- Preserves raw instruction string (for cache correctness) and instruction order
- Provides clear error messages with line numbers

### 🔹 Layer System
- Only `COPY` and `RUN` create layers
- Stored in `~/.docksmith/layers/`
- Deterministic: sorted files, zero timestamps
- Immutable, content-addressed (SHA-256)

### 🔹 Image System
- Stored in `~/.docksmith/images/`
- Includes: name, tag, digest, created (ISO-8601, stable), config (Env, Cmd, WorkingDir), ordered layers

### 🔹 Cache System
- Stored in `~/.docksmith/cache/`
- Cache key includes: previous layer digest, raw instruction, WORKDIR, ENV (sorted), COPY file hashes
- First build → CACHE MISS
- Rebuild → CACHE HIT
- Changes → cascade MISS
- `--no-cache` supported

### 🔹 Runtime (Linux / WSL2 Only)
- Extracts layers into temp rootfs
- Uses `chroot()` for isolation
- Applies working directory and environment variables
- Executes command inside container
- Cleans up after execution

---

## ⚠️ Important Requirements

- **Run Environment:** Must use Linux or WSL2 (Ubuntu). Do not use Docker, Windows CMD, or PowerShell.
- **Use `sudo` for runtime:** `chroot` requires elevated privileges.

---

## 🚀 How to Run

### 1. Build Image

```bash
go run ./cmd build -t test:latest .
```

### 2. Run Container

```bash
sudo go run ./cmd run test:latest
```

### 3. List Images

```bash
go run ./cmd images
```

### 4. Remove Image

```bash
go run ./cmd rmi test:latest
```

---

## 🧪 Example Build Output

```
Step 1/6 : FROM base
Step 2/6 : WORKDIR /app
Step 3/6 : COPY . /app [CACHE HIT/MISS]
Step 4/6 : ENV KEY=value
Step 5/6 : RUN echo hello [CACHE HIT/MISS]
Step 6/6 : CMD ["echo","hi"]
Successfully built sha256:xxx test:latest
```

---

## 🧪 Validation Summary

- **Build System:** Deterministic builds, correct layer creation
- **Cache:** HIT/MISS logic verified, cascade rule working
- **Runtime:** Executes commands correctly, uses isolated filesystem
- **Isolation (Critical Test):**
  - Test: `CMD ["sh","-c","echo hacked > /test.txt"]`
  - Result: `ls /test.txt` → No such file
  - ✔ No host filesystem access

---

## ⚠️ Strict Rules Followed

- Only `COPY` & `RUN` create layers
- Deterministic builds (same input → same digest)
- Cache uses full key (no shortcuts)
- No network usage
- Runtime fully isolated

---

## ❌ Common Mistakes Avoided

- Running commands on host
- Creating layers for ENV/WORKDIR
- Non-deterministic builds
- Incorrect cache reuse
- Isolation leaks

---

## 🎯 Final Outcome

This project successfully implements:

- Image builder
- Layered filesystem
- Deterministic cache system
- Isolated container runtime

👉 A working **mini Docker-like engine**

---

## 🏁 Demo Steps

1. Build (cold) → CACHE MISS
2. Build again → CACHE HIT
3. Run container → shows output
4. Isolation test → no host file creation
5. List images
6. Remove image

---

## 📌 Notes

- Always use the same user (`sudo` consistently) for build + run
- Runtime only works on Linux/WSL2 due to `chroot`

---

## 🎓 Conclusion

Docksmith Engine is a fully functional, deterministic, and isolated container system built from scratch in Go.
