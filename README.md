# Docksmith

Docksmith is a lightweight container/image tool with image builds, layer caching, and isolated runtime execution. Currently only works on Linux natively. MacOS support is limited and works only via a docker image. 
Every docksmith project needs a `docksmithfile` to build an image. Refer to `sample-app` in this repo to check how the file looks like.
## Requirements 
- go 1.24+

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

### 3) Add to PATH (Optional)

`/usr/local/bin` is usually already in PATH, but add it explicitly if needed.

#### Bash 

```bash
echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

#### Zsh
If your default is zsh, use this
```bash
echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.zshrc
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

## Example 
In this repo there is `sample-app` folder with the `docksmithfile` which has all the keywords supported by docksmith. To run it run follow the instructions below:

#### Get Alpine base image
Run the script file in `scripts` to get a test base image.

```bash
bash scripts/setup-base-image.sh
```
#### Building and running the image
To build the image in the `sample-app` folder, run the following command.

```bash
docksmith build -t demo:v1 sample-app
```
Running the image

```bash
sudo docksmith run demo:v1 
```
Your output should look something like this:
<img width="649" height="401" alt="image" src="https://github.com/user-attachments/assets/699092b7-5793-4b16-bb70-b5adf0589f86" />

## Additional commands
Docksmith supports more commands like container related commands such as `rm`, `ps` and `start`. To view them simply run `docksmith` in your cli.

