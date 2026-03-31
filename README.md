# Docksmith Engine

A simplified Docker-like build and runtime system.

---

##  Current Status (Gunadeep - CLI Orchestrator)

The CLI orchestration layer is **fully implemented and tested using mocks**.

### Completed:

* CLI commands:

	* `build`
	* `run`
	* `images`
	* `rmi`
* Argument parsing and validation
* Command routing
* Logging format (`[CACHE HIT]`, `[CACHE MISS]`)
* Interface contracts for all subsystems
* Mock implementations for testing full flow

### Important:

* CLI **DOES NOT contain business logic**
* All actual functionality must be implemented inside `internal/`

---

## ⚠️ DO NOT TOUCH (CRITICAL)

Do NOT modify:

* `cmd/main.go`
* `cmd/commands.go`
* `internal/interfaces.go`
* logging format

If you change interfaces or CLI flow:
👉 You will break integration for everyone

---

## 🧩 Work Allocation

| Member  | Module        |
| ------- | ------------- |
| Deepak  | Parser        |
| Chinmay | Layer + Image |
| Vishnu  | Cache         |
| ALL     | Runtime       |

---

## 🏗️ Architecture Flow

```
CLI → Parser → Builder → (Cache + Layer + Image)
										 ↓
									Runtime
```

---

## 📦 Environment Setup

### Install Go

#### macOS:

```bash
brew install go
```

#### Windows:

Download from:
https://go.dev/dl/

Verify:

```bash
go version
```

---

## 🐧 OS REQUIREMENT (IMPORTANT)

Runtime requires **Linux features (process isolation)**.

### macOS / Windows:

You MUST use:

* WSL2 (Windows)
* or Linux VM (VirtualBox / UTM)

👉 Do NOT attempt runtime directly on macOS/Windows host
👉 It will fail during demo (hard requirement)

---

## 🚀 How to Run CLI (Current Working State)

Using mocks:

```bash
go run ./cmd build -t test:latest .
go run ./cmd images
go run ./cmd run test:latest
go run ./cmd rmi test:latest
```

---

## 🧪 Expected Output

Example:

```
Step 1/3 : FROM base
Step 2/3 : COPY . /app [CACHE MISS]
Step 3/3 : RUN echo hello [CACHE MISS]
Successfully built sha256:dummy test:latest
```

---

## 🔧 Development Rules

### 1. NO NETWORK USAGE

* No internet access in build or run
* All dependencies must be local

---

### 2. STRICT INTERFACE USAGE

* Use only methods defined in `interfaces.go`
* Do NOT bypass CLI

---

### 3. DETERMINISTIC BUILDS

* Same input = same output
* Sort files
* Zero timestamps in tar

---

### 4. LAYER RULES

* COPY and RUN → create layers
* Layers must be immutable
* Content-addressed storage

---

### 5. CACHE RULES

* Exact key matching required
* Cache miss → cascade

---

## 🧪 Testing Strategy

### Use mocks first (already working)

Then replace with real implementations gradually.

---

## 🔥 Integration Plan

1. Parser ready → connect to CLI
2. Layer system ready → connect to Builder
3. Cache integrated into Builder
4. Runtime implemented last

---

## ❌ Common Mistakes (Avoid or Fail)

* Running container without isolation
* Using internet during build
* Changing interfaces mid-project
* Not sorting tar files
* Incorrect cache key

---

## 🧠 Final Goal

You are building:

* Image builder
* Cache system
* Container runtime

NOT a CLI tool.

CLI is already DONE.

---

## 📌 Next Step

Start implementing your module inside `internal/` and integrate via interfaces.

DO NOT modify CLI.
