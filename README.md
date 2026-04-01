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

	# Docksmith Engine

	A simplified Docker-like build and runtime system.

	---

	## ✅ Current Status (Gunadeep + Deepak + Chinmay)

	### 🔵 CLI Orchestrator — DONE (Gunadeep)

	* Commands: `build`, `run`, `images`, `rmi`
	* Argument parsing + validation
	* Strict logging (spec-compliant)
	* Clean orchestration (no business logic inside CLI)
	* Stable interfaces defined

	---

	### 🟢 Parser — DONE (Deepak)

	* Parses `Docksmithfile` into instructions
	* Supports:

		* FROM, COPY, RUN, WORKDIR, ENV, CMD
	* Strict validation with line numbers
	* Preserves:

		* Raw instruction string (CRITICAL for cache)
		* Instruction order
	* Handles whitespace + case normalization

	---

	### 🟡 Layer + Image System — DONE (Chinmay)

	#### Layer System:

	* ONLY `COPY` and `RUN` create layers
	* Stored in:

		```
		~/.docksmith/layers/
		```
	* Deterministic:

		* Sorted file order
		* Zero timestamps
	* SHA-256 content-addressed storage
	* Immutable layers

	#### Image Manifest:

	* Stored in:

		```
		~/.docksmith/images/
		```
	* Includes:

		* name, tag, digest
		* created timestamp (stable)
		* config (Env, Cmd, WorkingDir)
		* ordered layer list

	#### Verified:

	* Same build → SAME digest
	* File change → new digest (stable after change)
	* Correct layer count (COPY + RUN only)

	---

	## ⚠️ DO NOT TOUCH (CRITICAL)

	Do NOT modify:

	* `cmd/main.go`
	* `cmd/commands.go`
	* `internal/interfaces.go`
	* logging format
	* layer determinism logic

	If you change these:
	👉 You will break the entire system

	---

	## 🧩 Work Allocation (Updated)

	| Member   | Module           | Status  |
	| -------- | ---------------- | ------- |
	| Gunadeep | CLI Orchestrator | ✅ DONE  |
	| Deepak   | Parser           | ✅ DONE  |
	| Chinmay  | Layer + Image    | ✅ DONE  |
	| Vishnu   | Cache            | 🔥 NEXT |
	| ALL      | Runtime          | ⏳ FINAL |

	---

	## 🏗️ Architecture Flow

	```id="flow01"
	CLI → Parser → Builder → (Cache + Layer + Image)
											 ↓
										Runtime
	```

	---

	## 🚀 Current Working State

	```bash id="run01"
	go run ./cmd build -t test:latest .
	go run ./cmd run test:latest
	go run ./cmd images
	go run ./cmd rmi test:latest
	```

	---

	## 🧪 Expected Output

	```id="output01"
	Step 1/6 : FROM base
	Step 2/6 : WORKDIR /app
	Step 3/6 : COPY . /app [CACHE MISS]
	Step 4/6 : ENV KEY=value
	Step 5/6 : RUN echo hello [CACHE MISS]
	Step 6/6 : CMD ["echo","hi"]
	Successfully built sha256:xxx test:latest
	```

	---

	## ⚠️ STRICT RULES

	### 1. Layer Creation Rule

	* ONLY:

		* COPY
		* RUN
	* NO layers for:

		* FROM, WORKDIR, ENV, CMD

	---

	### 2. Determinism Rule

	* Same input → same digest
	* Required:

		* sorted tar entries
		* zero timestamps

	---

	### 3. Manifest Rule

	* `created` must:

		* be ISO-8601 format
		* remain SAME across rebuilds

	---

	### 4. Logging Rule

	* Cache logs ONLY for:

		* COPY
		* RUN
	* No cache logs for others

	---

	### 5. NO NETWORK

	* No internet during build or run

	---

	## 🔥 NEXT TASK — Vishnu (Cache System)

	### ⚠️ IMPORTANT

	Current `[CACHE MISS]` is FAKE

	Real cache must:

	* Compute deterministic cache key
	* Reuse layers when key matches
	* Follow strict invalidation rules

	---

	## ❌ COMMON MISTAKES (CRITICAL)

	* Creating layers for all instructions ❌
	* Non-deterministic tar files ❌
	* Changing `created` timestamp every build ❌
	* Wrong cache key logic ❌
	* Ignoring instruction order ❌

	---

	## 🧪 Testing Strategy

	Already verified:

	* CLI ✔
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
