package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

// ContainerStatus is the display model for `docksmith ps`.
type ContainerStatus struct {
	ID         string
	Name       string
	Image      string
	Command    []string
	Running    bool
	PID        int
	ExitCode   *int
	StartedAt  *time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
}

// ListContainerStatuses returns all known containers from config.json files.
func ListContainerStatuses(containersDir string) ([]ContainerStatus, error) {
	if err := os.MkdirAll(containersDir, 0755); err != nil {
		return nil, fmt.Errorf("create containers dir: %w", err)
	}

	entries, err := os.ReadDir(containersDir)
	if err != nil {
		return nil, fmt.Errorf("read containers dir: %w", err)
	}

	result := make([]ContainerStatus, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cfgPath := filepath.Join(containersDir, e.Name(), "config.json")
		raw, readErr := os.ReadFile(cfgPath)
		if readErr != nil {
			continue
		}

		var cfg containerConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			continue
		}

		running := false
		if cfg.PID > 0 {
			running = processExists(cfg.PID)
		}

		if cfg.Name == "" {
			cfg.Name = "ctr-" + cfg.ID
		}

		result = append(result, ContainerStatus{
			ID:         cfg.ID,
			Name:       cfg.Name,
			Image:      cfg.Image,
			Command:    cfg.Command,
			Running:    running,
			PID:        cfg.PID,
			ExitCode:   cfg.ExitCode,
			StartedAt:  cfg.StartedAt,
			FinishedAt: cfg.FinishedAt,
			CreatedAt:  cfg.Created,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Running != result[j].Running {
			return result[i].Running
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

