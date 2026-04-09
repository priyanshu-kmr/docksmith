# Docksmith — Demo

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

> [NOTE]
> If you've already done this before, the script will say "Base image alpine:3.18 already exists" and exit. To re-import, delete the manifest first: `rm ~/.docksmith/images/alpine_3.18.json`

---

## Demo

### Step 1 — Cold Build (all CACHE MISS)

```bash
./docksmith build -t myapp:latest sample-app/
```

---

### Step 2 — Warm Build (all CACHE HIT)

```bash
./docksmith build -t myapp:latest sample-app/
```

---

### Step 3 — Cache Invalidation (edit source, partial hit)

```bash
echo "changed" >> sample-app/message.txt
./docksmith build -t myapp:latest sample-app/
```

Revert the change:
```bash
git checkout sample-app/message.txt
```

---

### Step 4 — List Images

```bash
./docksmith images
```

**Note:**
- ID is first 12 characters of the SHA-256 digest
- Both the base image and built image are shown

---

### Step 5 — Run Container

```bash
./docksmith run myapp:latest
```
---

### Step 6 — ENV Override with `-e`

```bash
./docksmith run -e GREETING=Goodbye myapp:latest
```

Multiple `-e` flags can be used: `-e KEY1=val1 -e KEY2=val2`

---

### Step 7 — Verify Filesystem Isolation

```bash
./docksmith run myapp:latest sh -c "echo 'SECRET' > /tmp/test_isolation && echo 'Written inside container'"
```

Then check the host:
```bash
cat /tmp/test_isolation
```

- The file was written inside the container's isolated root filesystem
- After the container exits, the temporary rootfs is cleaned up
- Isolation uses Linux namespaces (`CLONE_NEWPID | NEWNS | NEWUTS | NEWUSER`) + `chroot`
---

### Step 8 — Remove Image

```bash
./docksmith rmi myapp:latest
```

Verify it's gone:
```bash
./docksmith images
```

---

## Architecture Summary

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
