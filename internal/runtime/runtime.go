package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// RunConfig holds configuration for an isolated process.
type RunConfig struct {
	RootFS   string            // Path to the assembled rootfs directory
	Command  []string          // Command and arguments to execute
	Env      map[string]string // Environment variables
	WorkDir  string            // Working directory inside the container (default "/")
}

// RunIsolated executes a command inside an isolated environment.
// This is the single isolation primitive used by both RUN during build
// and docksmith run at runtime.
//
// It re-executes the current binary with a special "_child" argument
// to perform the actual chroot + exec inside new namespaces.
func RunIsolated(cfg RunConfig) (int, error) {
	if len(cfg.Command) == 0 {
		return 1, fmt.Errorf("no command specified")
	}
	if cfg.RootFS == "" {
		return 1, fmt.Errorf("rootfs path is required")
	}
	if cfg.WorkDir == "" {
		cfg.WorkDir = "/"
	}

	// Prepare /proc, /dev, /sys mount points inside rootfs
	for _, dir := range []string{"proc", "dev", "sys", "tmp"} {
		target := filepath.Join(cfg.RootFS, dir)
		if err := os.MkdirAll(target, 0755); err != nil {
			return 1, fmt.Errorf("create %s: %w", dir, err)
		}
	}

	// Also ensure the workdir exists inside rootfs
	workdirPath := filepath.Join(cfg.RootFS, cfg.WorkDir)
	if err := os.MkdirAll(workdirPath, 0755); err != nil {
		return 1, fmt.Errorf("create workdir %s: %w", cfg.WorkDir, err)
	}

	// Build environment slice
	envSlice := make([]string, 0, len(cfg.Env)+2)
	envSlice = append(envSlice, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	envSlice = append(envSlice, "HOME=/root")
	for k, v := range cfg.Env {
		envSlice = append(envSlice, k+"="+v)
	}

	// We re-exec ourselves with a special sentinel argument.
	// The child process will run inside new namespaces.
	self, err := os.Executable()
	if err != nil {
		return 1, fmt.Errorf("get executable path: %w", err)
	}

	// Encode the config into child args:
	// _child <rootfs> <workdir> <cmd...>
	childArgs := []string{"_child", cfg.RootFS, cfg.WorkDir}
	childArgs = append(childArgs, cfg.Command...)

	cmd := exec.Command(self, childArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = envSlice

	// Create new namespaces for isolation
	// CLONE_NEWUSER allows running without root by mapping current UID/GID
	// to root inside the container.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWUSER,
		Unshareflags: syscall.CLONE_NEWNS,
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		},
	}

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("run isolated process: %w", err)
	}

	return 0, nil
}

// ChildMain is called when the binary is re-executed with "_child" as the
// first argument. It runs inside new namespaces and performs the actual
// chroot + exec.
func ChildMain(args []string) {
	// args: [_child, rootfs, workdir, cmd...]
	if len(args) < 4 {
		fmt.Fprintf(os.Stderr, "child: insufficient arguments\n")
		os.Exit(1)
	}

	rootFS := args[1]
	workDir := args[2]
	command := args[3:]

	if err := childRun(rootFS, workDir, command); err != nil {
		fmt.Fprintf(os.Stderr, "container error: %v\n", err)
		os.Exit(1)
	}
}

func childRun(rootFS, workDir string, command []string) error {
	// Mount proc
	procPath := filepath.Join(rootFS, "proc")
	if err := syscall.Mount("proc", procPath, "proc", 0, ""); err != nil {
		// Non-fatal: some environments may not support this
		fmt.Fprintf(os.Stderr, "warning: mount /proc: %v\n", err)
	}

	// Mount a minimal /dev with devtmpfs or tmpfs
	devPath := filepath.Join(rootFS, "dev")
	if err := syscall.Mount("tmpfs", devPath, "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: mount /dev: %v\n", err)
	} else {
		// Create essential device nodes
		createDevNodes(devPath)
	}

	// Mount tmpfs on /tmp
	tmpPath := filepath.Join(rootFS, "tmp")
	if err := syscall.Mount("tmpfs", tmpPath, "tmpfs", 0, ""); err != nil {
		fmt.Fprintf(os.Stderr, "warning: mount /tmp: %v\n", err)
	}

	// Pivot root: use chroot for simplicity and broad compatibility
	if err := syscall.Chroot(rootFS); err != nil {
		return fmt.Errorf("chroot: %w", err)
	}
	if err := os.Chdir(workDir); err != nil {
		return fmt.Errorf("chdir to %s: %w", workDir, err)
	}

	// Set hostname
	_ = syscall.Sethostname([]byte("docksmith"))

	// Find the executable
	cmdPath, err := lookPath(command[0])
	if err != nil {
		return fmt.Errorf("command not found: %s", command[0])
	}

	// Exec the command — replaces this process
	return syscall.Exec(cmdPath, command, os.Environ())
}

// lookPath searches for an executable in PATH after chroot.
func lookPath(name string) (string, error) {
	if strings.Contains(name, "/") {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
		return "", fmt.Errorf("%s: not found", name)
	}

	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		pathEnv = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	}

	for _, dir := range strings.Split(pathEnv, ":") {
		full := filepath.Join(dir, name)
		if info, err := os.Stat(full); err == nil {
			if info.Mode()&0111 != 0 {
				return full, nil
			}
		}
	}

	return "", fmt.Errorf("%s: not found in PATH", name)
}

// createDevNodes creates essential /dev entries.
func createDevNodes(devPath string) {
	nodes := []struct {
		name  string
		mode  uint32
		major uint32
		minor uint32
	}{
		{"null", syscall.S_IFCHR | 0666, 1, 3},
		{"zero", syscall.S_IFCHR | 0666, 1, 5},
		{"random", syscall.S_IFCHR | 0666, 1, 8},
		{"urandom", syscall.S_IFCHR | 0666, 1, 9},
		{"tty", syscall.S_IFCHR | 0666, 5, 0},
	}

	for _, n := range nodes {
		path := filepath.Join(devPath, n.name)
		dev := int(n.major*256 + n.minor)
		_ = syscall.Mknod(path, n.mode, dev)
	}
}
