# Docksmith Engine

A simplified Docker-like build and runtime system.

---

## ✅ Current Status (Gunadeep + Deepak)

### 🔵 CLI Orchestrator — COMPLETED (Gunadeep)

* CLI commands: `build`, `run`, `images`, `rmi`
* Argument parsing and validation
* Command routing (Parser → Builder → Runtime)
* Strict logging format (spec-compliant)
* Interface contracts defined
* Mock system replaced progressively

---


## ✅ Current Status

### 🔵 CLI Orchestrator — DONE

- Commands: `build`, `run`, `images`, `rmi`
- Argument parsing + validation
- Strict logging format (cache/timing only on `COPY` and `RUN`)

### 🟢 Parser — DONE

- Parses `Docksmithfile` into instructions: `FROM`, `COPY`, `RUN`, `WORKDIR`, `ENV`, `CMD`
- Strict validation with line-numbered errors
- Preserves exact raw instruction text (used for caching)

### 🟡 Layer + Image System — DONE

- Only `COPY` and `RUN` create layers
- Deterministic outputs (sorted entries, normalized metadata)
- Stores layers in `~/.docksmith/layers/` and manifests in `~/.docksmith/images/`
- Manifest `created` is stable across identical rebuilds (digest remains stable)

### 🟣 Cache System — DONE

- Real cache storage: `~/.docksmith/cache/`
- Real `[CACHE HIT]` / `[CACHE MISS]` reporting for `COPY` and `RUN`
- Cascade invalidation works (a miss forces downstream misses)
- Cache keys include raw formatting changes as required

#### Cache Protocol Verification (PHASE 1–7)

- Clean build → MISS, rebuild unchanged → HIT
- Context file change → COPY MISS and cascade RUN MISS
- Revert file → HIT again
- ENV/WORKDIR changes → COPY HIT, RUN MISS
- Raw formatting change → MISS
- `--no-cache` → always MISS

---

## 🏗️ Architecture Flow

CLI → Parser → Builder → (Cache + Layer + Image) → Runtime

---

## 🚀 Quick Start

```bash
go run ./cmd build -t test:latest .
go run ./cmd run test:latest
go run ./cmd images
go run ./cmd rmi test:latest
```
	* Parser ✔
	* Layer/Image ✔

	Next:

	* Cache verification (critical)
	* Runtime validation (final)

	---

	## 🧠 Final Goal

	You are building:

	* Image builder
	* Deterministic cache system
	* Container runtime with isolation

	NOT just a CLI tool.

	---

	## 📌 Current Priority

	👉 Vishnu must implement CACHE SYSTEM
	👉 Do NOT start runtime before cache is correct

	---

	## ⚠️ Final Warning

	If cache is wrong:

	* builds will not reuse layers
	* system becomes inefficient
	* demo will fail

	Implement carefully.
