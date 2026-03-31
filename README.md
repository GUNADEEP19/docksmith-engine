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

### 🟢 Parser — COMPLETED (Deepak)

* Parses `Docksmithfile` into structured instructions
* Supports all 6 instructions:

	* `FROM`
	* `COPY`
	* `RUN`
	* `WORKDIR`
	* `ENV`
	* `CMD`
* Strict validation:

	* Unknown instructions → error with line number
	* Invalid ENV → rejected
	* Invalid CMD → rejected (must be JSON array)
* Preserves:

	* **Raw instruction string (CRITICAL for cache)**
	* Order of instructions
* Handles:

	* Whitespace variations
	* Case normalization (internally)

---

## ⚠️ DO NOT TOUCH (CRITICAL)

Do NOT modify:

* `cmd/main.go`
* `cmd/commands.go`
* `internal/interfaces.go`
* logging format

If you change interfaces or CLI flow:
👉 You will break integration for everyone

## 🧩 Work Allocation (Updated)

| Member   | Module           | Status    |
| -------- | ---------------- | --------- |
| Gunadeep | CLI Orchestrator | ✅ DONE    |
| Deepak   | Parser           | ✅ DONE    |
| Chinmay  | Layer + Image    | 🔜 NEXT   |
| Vishnu   | Cache            | ⏳ PENDING |
| ALL      | Runtime          | 🔥 FINAL  |

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
										 ↓
									Runtime
brew install go
```

## 🚀 Current Working State

Parser is fully integrated with CLI.

### Run:
## 🧪 Expected Output

Example:
```
Step 2/3 : COPY . /app [CACHE MISS]
Step 3/3 : RUN echo hello [CACHE MISS]
Successfully built sha256:dummy test:latest

## 🧪 Expected Output (Correct Format)
## 🔧 Development Rules

Step 1/6 : FROM base
Step 2/6 : WORKDIR /app
Step 3/6 : COPY . /app [CACHE MISS] 0.02s
Step 4/6 : ENV KEY=value
Step 5/6 : RUN echo hello [CACHE MISS] 0.04s
Step 6/6 : CMD ["echo","hi"]
* All dependencies must be local

---


## ⚠️ IMPORTANT RULES (STRICT)

### 1. Cache Logging Rule

* ONLY `COPY` and `RUN` show:

	* `[CACHE HIT]` / `[CACHE MISS]`
	* execution time

* `FROM`, `WORKDIR`, `ENV`, `CMD`:

	* ❌ NO cache status
	* ❌ NO timing
### 2. STRICT INTERFACE USAGE
---

### 2. NO NETWORK USAGE

* No internet access in build or run
* All dependencies must be local

---

### 3. STRICT INTERFACE USAGE

* Use only methods defined in `interfaces.go`
* Do NOT bypass CLI

---

### 4. DETERMINISTIC BUILDS

* Same input = same output
* Sort files
* Zero timestamps in tar

---

## 🔥 NEXT TASK — Chinmay (Layer + Image System)

### Your Job:

Implement:

* `internal/layer/`
* `internal/image/`

---

### Requirements:

#### Layer System

* COPY and RUN → create layers
* Store as tar files:

	```
	~/.docksmith/layers/<digest>.tar
	```
* Compute SHA-256 digest of tar
* Must be deterministic

---

#### Image Manifest

* Store JSON in:

	```
	~/.docksmith/images/
	```
* Include:

	* name, tag
	* digest
	* config (Env, Cmd, WorkingDir)
	* layers list

---

#### Extraction

* Apply layers in order
* Later layers overwrite earlier ones

---

## ❌ COMMON MISTAKES (DO NOT DO THIS)

* Creating full snapshot instead of delta layer
* Not sorting tar entries
* Including timestamps in tar
* Incorrect SHA-256 calculation
* Modifying layer after creation
* Ignoring instruction order

---

## 🧪 HOW TO VERIFY (Layer System)

1. Same build twice → SAME digest
2. Change one file → ONLY one layer changes
3. Extract layers → correct final filesystem

---

## 🧪 Testing Strategy

* CLI + Parser already verified
* Replace mocks gradually
* Test each module independently before integration

---

## 🧠 Final Goal

You are building:

* Image builder
* Cache system
* Container runtime

NOT just CLI or parser.

---

## 📌 Current Priority

👉 Chinmay must complete Layer + Image system
👉 DO NOT start cache or runtime before this

---

## ⚠️ Final Warning

If Layer system is incorrect:

* Cache will fail
* Runtime will fail
* Entire project collapses

Build it carefully.
