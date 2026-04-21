# Docksmith — Essential commands

This file contains the minimal commands you need to run the sample demo in this repository (WSL/Linux). Each command is followed by the expected output and a short explanation.

Notes:
- Use WSL or a Linux environment. `build`/`run` use `chroot` and must be run as root (sudo) unless you change platform permissions.
- Run these commands from the repository root.

---

## 0) Import the one-time base image (offline)
```bash
sudo go run ./cmd base base:latest
```
Expected output:
```text
Imported base image base:latest
```
Explanation: creates a base image manifest under the Docksmith data directory (root's `~/.docksmith` when run with sudo). Do this once before building images that `FROM base:latest`.

---

## 1) Cold build (first time; expect cache MISS)
```bash
sudo go run ./cmd build -t sample:latest ./sample-app
```
Expected output (representative):
```text
Step 1/6 : FROM base:latest
Step 2/6 : WORKDIR /app
Step 3/6 : COPY . /app [CACHE MISS] 0.12s
Step 4/6 : ENV MESSAGE=Hello
Step 5/6 : RUN sh -c "echo built > /app/build.txt" [CACHE MISS] 0.50s
Step 6/6 : CMD ["sh","/app/app/main.sh"]
Successfully built sha256:<digest> sample:latest
```
Explanation: builds the image from `./sample-app` and stores the manifest and layer blobs under `~/.docksmith`. Use `--no-cache` to force rebuild of all steps.

---

## 2) Warm build (cache HIT)
```bash
sudo go run ./cmd build -t sample:latest ./sample-app
```
Expected output (representative):
```text
Step 1/6 : FROM base:latest
Step 2/6 : WORKDIR /app
Step 3/6 : COPY . /app [CACHE HIT] 0.03s
Step 4/6 : ENV MESSAGE=Hello
Step 5/6 : RUN sh -c "echo built > /app/build.txt" [CACHE HIT] 0.00s
Step 6/6 : CMD ["sh","/app/app/main.sh"]
Successfully built sha256:<digest> sample:latest
```
Explanation: repeated build uses the deterministic cache; COPY/RUN steps may show HIT or MISS depending on context changes.

---

## 3) Run the image (execute CMD)
```bash
sudo go run ./cmd run sample:latest
```
Expected output:
```text
[run] image=sample:latest cmd=sh /app/app/main.sh
Hello from Docksmith sample app!
Working directory: /app
MESSAGE=Hello
Build artifact: built
Exit code: 0
```
Explanation: runs the image in a chroot-based runtime and prints the script output. The first `[` line is the CLI log; the following lines are the script's stdout. `Exit code: 0` indicates success.

---

## 4) Run with ENV override
```bash
sudo go run ./cmd run -e MESSAGE="Hi Professor" sample:latest
```
Expected output (representative):
```text
[run] image=sample:latest cmd=sh /app/app/main.sh
Hello from Docksmith sample app!
Working directory: /app
MESSAGE=Hi Professor
Build artifact: built
Exit code: 0
```
Explanation: override container env variables for the run.

---

## 5) List local images
```bash
sudo go run ./cmd images
```
Expected output (representative):
```text
NAME	TAG	ID
sample	latest	sha256:<digest>
```
Explanation: lists manifests in your current user's Docksmith data store (`~/.docksmith/images`). If you built with `sudo`, run this with `sudo` to inspect root's store.

---

## 6) Remove an image
```bash
sudo go run ./cmd rmi sample:latest
```
Expected output:
```text
Removed image sample:latest
```
Explanation: delete the manifest and any layer files referenced by it. Use the same user you used to create/save the image (sudo vs non-sudo) so the CLI inspects the same data root.

---

## 7) Force rebuild (skip cache)
```bash
sudo go run ./cmd build -t sample:latest --no-cache ./sample-app
```
Expected output: similar to a cold build; COPY and RUN will show `[CACHE MISS]`.

---

## 8) Fix CRLF line endings in scripts (if you see `set: Illegal option -` or `^M` in output)
If your editor/Windows checkout introduced CRLF line endings, convert scripts to LF before building. Example (run in WSL):
```bash
# convert in-place
sed -i 's/\r$//' sample-app/app/main.sh
# verify no ^M shown
sed -n l sample-app/app/main.sh
```
Expected result: script displays without `^M` at line ends; rebuild/run should no longer error with `set: Illegal option -`.

---

If you prefer, add `.gitattributes` to the repo to force LF for shell scripts on checkout:
```
*.sh text eol=lf
```

That's it — these are the minimal, reproducible commands to import the base, build the sample image, run it, list and remove images, and fix the common CRLF issue.
