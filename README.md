# Docksmith Engine

A simplified Docker-like build and runtime system.

---

## ✅ Current Status (Gunadeep + Deepak + Chinmay + Vishnu)

### 🔵 CLI Orchestrator — DONE (Gunadeep)

* Commands: `build`, `run`, `images`, `rmi`
* Argument parsing and validation
* Clean orchestration (no business logic)
* Strict logging (spec-compliant)

---

### 🟢 Parser — DONE (Deepak)

* Parses Docksmithfile into structured instructions
* Supports:

	* FROM, COPY, RUN, WORKDIR, ENV, CMD
* Strict validation with line numbers
* Preserves:

	* Raw instruction string (CRITICAL for cache)
	* Instruction order
* Handles whitespace and case normalization

---

### 🟡 Layer + Image System — DONE (Chinmay)

#### Layer System:

* ONLY COPY and RUN create layers
* Stored in:
	~/.docksmith/layers/
* Deterministic:

	* Sorted file order
	* Zero timestamps
* SHA-256 content-addressed
* Immutable layers

#### Image Manifest:

* Stored in:
	~/.docksmith/images/
* Includes:

	* name, tag, digest
	* created timestamp (ISO-8601, stable)
	* config (Env, Cmd, WorkingDir)
	* ordered layer list

#### Verified:

* Same build → SAME digest
* File change → new digest (stable)
* Correct layer count (COPY + RUN only)

---

### 🔴 Cache System — DONE (Vishnu)

#### Cache Key:

Includes:

* previous layer digest
* raw instruction string
* WORKDIR
* ENV (sorted)
* COPY source file hashes

#### Behavior:

* First build → CACHE MISS
* Rebuild → CACHE HIT
* Change → cascade MISS
* ENV/WORKDIR changes invalidate downstream
* `--no-cache` forces MISS

#### Storage:

~/.docksmith/cache/

* key → layer digest mapping

#### Verified:

* Deterministic cache behavior
* Correct cascade rule
* Raw instruction sensitivity
* Proper invalidation

---

## 🧩 Work Allocation (Final Stage)

| Member   | Module           | Status  |
| -------- | ---------------- | ------- |
| Gunadeep | CLI Orchestrator | ✅ DONE  |
| Deepak   | Parser           | ✅ DONE  |
| Chinmay  | Layer + Image    | ✅ DONE  |
| Vishnu   | Cache            | ✅ DONE  |
| ALL      | Runtime          | ✅ DONE (Linux/WSL2) |

---

## 🏗️ Architecture Flow

```id="flow02"
CLI → Parser → Builder → (Cache + Layer + Image)
										 ↓
									Runtime
```

---

## 🚀 Current Working Commands

```bash id="cmds01"
go run ./cmd build -t test:latest .
go run ./cmd images
sudo go run ./cmd run test:latest   # Linux/WSL2 only (chroot isolation)
go run ./cmd rmi test:latest
```

---

## 🧪 Expected Build Output

```id="output02"
Step 1/6 : FROM base
Step 2/6 : WORKDIR /app
Step 3/6 : COPY . /app [CACHE HIT/MISS]
Step 4/6 : ENV KEY=value
Step 5/6 : RUN echo hello [CACHE HIT/MISS]
Step 6/6 : CMD ["echo","hi"]
Successfully built sha256:xxx test:latest
```

---

## ⚠️ STRICT RULES (DO NOT BREAK)

### 1. Layer Rules

* ONLY COPY and RUN create layers
* Others update config only

---

### 2. Determinism

* Same input → same digest
* Must:

	* sort files
	* zero timestamps

---

### 3. Cache Rules

* Full key match required
* Cascade rule must hold
* Raw instruction MUST be used

---

### 4. Manifest Rules

* `created` must:

	* be ISO-8601
	* remain unchanged across rebuilds

---

### 5. Logging Rules

* Cache logs ONLY for COPY and RUN

---

### 6. NO NETWORK

* No internet usage during build or run

---

## 🧱 Runtime — DONE (Linux/WSL2)

### Goal:

Run container with full filesystem isolation.

---

### Required:

1. Extract layers into temp directory
2. Set working directory
3. Apply environment variables
4. Execute command inside isolated root
5. Clean up after execution

---

### CRITICAL REQUIREMENT:

👉 Container MUST NOT access host filesystem

---

### Must Use:

* chroot() OR Linux namespaces

---

## 🧪 Runtime Verification (IMPORTANT)

### Test 1 — Basic run

* command executes correctly

---

### Test 2 — Isolation (PASS/FAIL)

Inside container:

```
echo "hello" > /test.txt
```

After run:

* file MUST NOT exist on host

---

### Test 3 — ENV override

* -e KEY=value must override

---

### Test 4 — Working directory

* commands run in correct path

---

## ❌ COMMON FAILURE POINTS

* Not isolating filesystem
* Running commands on host
* Incorrect layer extraction order
* Not cleaning temp directory
* Weak ENV handling

---

## 🧠 Final Goal

You are building:

* Deterministic image builder
* Correct cache system
* Isolated container runtime

---

## 📌 Current Status

👉 Build system is COMPLETE
👉 Runtime is FINAL step

---

## ⚠️ Final Warning

If runtime is wrong:

* demo = FAIL immediately

Isolation is a **pass/fail requirement**

Implement carefully.
