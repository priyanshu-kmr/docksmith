# Docksmith

Docksmith is a lightweight container/image tool with image builds, layer caching, and isolated runtime execution. Currently only works on Linux natively. MacOS support is limited and works only via a docker image. 

## Requirements 
- go 1.26.1

## Setup

### 1) Build the binary

From the repository root:

```bash
go build -o docksmith .
```

### 2) Install globally 

Install to `/usr/local/bin` so it is available system-wide:

```bash
sudo install -m 0755 docksmith /usr/local/bin/docksmith
```

If SELinux is enabled (eg.Fedora), apply the proper label:

```bash
sudo restorecon -v /usr/local/bin/docksmith || true
```

### 3) Add to PATH

`/usr/local/bin` is usually already in PATH, but add it explicitly if needed.

#### Bash 

```bash
grep -q 'export PATH="/usr/local/bin:$PATH"' ~/.bashrc || \
  printf '\n# docksmith path\nexport PATH="/usr/local/bin:$PATH"\n' >> ~/.bashrc

source ~/.bashrc
```

#### Zsh (Optional)
If your default is zsh, use this
```bash
grep -q 'export PATH="/usr/local/bin:$PATH"' ~/.zshrc || \
  printf '\n# docksmith path\nexport PATH="/usr/local/bin:$PATH"\n' >> ~/.zshrc

source ~/.zshrc
```

### 4) Verify

```bash
command -v docksmith
docksmith images
```

To verify sudo path resolution too:

```bash
sudo command -v docksmith
```

## Rebuild after code changes

If you change the source code, rebuild and reinstall:

```bash
go build -o docksmith .
sudo install -m 0755 docksmith /usr/local/bin/docksmith
```

go build will only update `./docksmith`, not `/usr/local/bin/docksmith`.


## Commands

run `docksmith` in your shell to see list of commands. Add `sudo` before every command. Eg ` sudo docksmith build -t demo:v1 sample-app`
