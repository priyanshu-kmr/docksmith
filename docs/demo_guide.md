# Docksmith — Step-by-Step Demo Guide (OLD)

## Prerequisites

- **Linux** (native or WSL2)
- **Go 1.26+** installed
- Internet access (one-time, for base image download only)

---

## Setup (One-Time)

### 1. Build the binary

```bash
cd ~/Desktop/Projects/docksmith
go build -o docksmith .
```

### 2. Import the base image

This downloads Alpine Linux and imports it into `~/.docksmith/`. Run once — everything after this works **fully offline**.

```bash
bash scripts/setup-base-image.sh
```

> [!NOTE]
> If you've already done this before, the script will say "Base image alpine:3.18 already exists" and exit. To re-import, delete the manifest first: `rm ~/.docksmith/images/alpine_3.18.json`

---

## Demo Flow (8 Steps)

### Step 1 — Cold Build (all CACHE MISS)

```bash
./docksmith build -t myapp:latest sample-app/
```

**Expected output:**
```
Step 1/7 : FROM alpine:3.18
Step 2/7 : WORKDIR /app
Step 3/7 : ENV APP_NAME=docksmith-demo
Step 4/7 : ENV GREETING=Hello
Step 5/7 : COPY . /app [CACHE MISS] 0.00s
Step 6/7 : RUN sh -c "echo 'Build complete at startup' > /app/build.log && chmod +x /app/run.sh" [CACHE MISS] 0.09s
Step 7/7 : CMD ["sh", "/app/run.sh"]
Successfully built sha256:XXXXXXXXXXXX myapp:latest (0.11s)
```

**What to point out:**
- All 6 instructions are used: `FROM`, `WORKDIR`, `ENV`, `COPY`, `RUN`, `CMD`
- Only `COPY` and `RUN` produce layers (shown with cache status + timing)
- `FROM`, `WORKDIR`, `ENV`, `CMD` have no cache status — they're config-only

---

### Step 2 — Warm Build (all CACHE HIT)

```bash
./docksmith build -t myapp:latest sample-app/
```

**Expected output:**
```
Step 5/7 : COPY . /app [CACHE HIT] 0.00s
Step 6/7 : RUN ... [CACHE HIT] 0.00s
Successfully built sha256:XXXXXXXXXXXX myapp:latest (0.00s)
```

**What to point out:**
- Same digest as cold build — **byte-for-byte reproducible**
- Near-instant (0.00s) because nothing is re-executed
- `created` timestamp is preserved from the first build

---

### Step 3 — Cache Invalidation (edit source, partial hit)

```bash
echo "changed" >> sample-app/message.txt
./docksmith build -t myapp:latest sample-app/
```

**Expected output:**
```
Step 5/7 : COPY . /app [CACHE MISS] 0.00s
Step 6/7 : RUN ... [CACHE MISS] 0.08s
```

**What to point out:**
- `COPY` is a miss because source file content changed
- `RUN` is also a miss because **cache miss cascades downstream**
- Digest changes (different from steps 1–2)

Revert the change:
```bash
git checkout sample-app/message.txt
```

---

### Step 4 — List Images

```bash
./docksmith images
```

**Expected output:**
```
NAME                     TAG          ID             CREATED
alpine                   3.18         bdba6400219e   2026-04-09T04:38:04Z
myapp                    latest       f77f51734573   2026-04-09T...
```

**What to point out:**
- ID is first 12 characters of the SHA-256 digest
- Both the base image and built image are shown

---

### Step 5 — Run Container

```bash
./docksmith run myapp:latest
```

**Expected output:**
```
================================
  Hello from docksmith-demo!
================================

Environment:
  APP_NAME = docksmith-demo
  GREETING = Hello
  PWD      = /app

Build log:
Build complete at startup

Message:
Docksmith is running inside an isolated container!
...
Container running successfully!
Container exited with code 0
```

**What to point out:**
- `ENV` values (`APP_NAME`, `GREETING`) are injected into the container
- `WORKDIR` is `/app` (shown by `PWD`)
- `CMD` runs the `run.sh` script
- Container exits cleanly

---

### Step 6 — ENV Override with `-e`

```bash
./docksmith run -e GREETING=Goodbye myapp:latest
```

**Expected output:**
```
================================
  Goodbye from docksmith-demo!
================================

Environment:
  GREETING = Goodbye
  ...
```

**What to point out:**
- `-e GREETING=Goodbye` overrides the image's `ENV GREETING=Hello`
- Multiple `-e` flags can be used: `-e KEY1=val1 -e KEY2=val2`

---

### Step 7 — Verify Filesystem Isolation (PASS/FAIL)

```bash
./docksmith run myapp:latest sh -c "echo 'SECRET' > /tmp/test_isolation && echo 'Written inside container'"
```

Then check the host:
```bash
cat /tmp/test_isolation 2>/dev/null && echo "FAIL: file leaked!" || echo "PASS: file does NOT exist on host"
```

**Expected:** `PASS: file does NOT exist on host`

**What to point out:**
- The file was written inside the container's isolated root filesystem
- After the container exits, the temporary rootfs is cleaned up
- **No file appears on the host** — this is the hard pass/fail criterion
- Isolation uses Linux namespaces (`CLONE_NEWPID | NEWNS | NEWUTS | NEWUSER`) + `chroot`

---

### Step 8 — Remove Image

```bash
./docksmith rmi myapp:latest
```

**Expected output:**
```
Removed image myapp:latest
```

Verify it's gone:
```bash
./docksmith images
```

Only `alpine:3.18` should remain. The manifest and all layer tar files belonging to `myapp:latest` are deleted from `~/.docksmith/`.

---

## Bonus Demos

### `--no-cache` Flag
```bash
./docksmith build -t myapp:latest --no-cache sample-app/
```
All steps show `[CACHE MISS]` regardless of whether cached layers exist. No cache entries are written.

### Command Override
```bash
./docksmith run myapp:latest echo "Custom command"
```
Overrides the image's `CMD` with a custom command.

### Inspect the State Directory
```bash
ls ~/.docksmith/images/    # JSON manifests
ls ~/.docksmith/layers/    # Content-addressed tar files
ls ~/.docksmith/cache/     # Cache index entries
cat ~/.docksmith/images/myapp_latest.json | python3 -m json.tool
```

---

## Architecture Summary (for Q&A)

| Component | What it does |
|-----------|-------------|
| **CLI** (`main.go`) | Routes `build`, `images`, `rmi`, `run` commands |
| **Parser** (`internal/parser/`) | Reads `Docksmithfile`, validates 6 instructions |
| **Build Engine** (`internal/build/`) | Executes instructions, manages layers, writes manifests |
| **Cache** (`internal/cache/`) | Deterministic key computation, disk-backed storage |
| **Layer Store** (`internal/layer/`) | Content-addressed tar storage, deterministic tar creation |
| **Image Store** (`internal/image/`) | Manifest JSON read/write, digest computation |
| **Runtime** (`internal/runtime/`) | `RunIsolated()` — one primitive used for both build `RUN` and `docksmith run` |

**Key design point:** `RunIsolated()` is the single isolation primitive. It re-execs the binary with `_child` argument inside new namespaces, then does `chroot` + `exec`. Used identically for build-time `RUN` and runtime `docksmith run`.
