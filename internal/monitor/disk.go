package monitor

import (
	"fmt"
	"syscall"
)

type DiskMount struct {
	MountPoint string  `json:"mount_point"`
	TotalGB    float64 `json:"total_gb"`
	UsedGB     float64 `json:"used_gb"`
	Usage      float64 `json:"usage"`
}

type DiskResult struct {
	Mounts []DiskMount `json:"mounts"`
	Alert  bool        `json:"alert"`
}

func CheckDisk(threshold int, paths []string) (*DiskResult, error) {
	if len(paths) == 0 {
		paths = []string{"/"}
	}

	result := &DiskResult{
		Alert: false,
	}

	for _, path := range paths {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(path, &stat); err != nil {
			return nil, fmt.Errorf("statfs %s: %w", path, err)
		}

		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bfree * uint64(stat.Bsize)
		used := total - free

		var usage float64
		if total > 0 {
			usage = float64(used) / float64(total) * 100
		}

		mount := DiskMount{
			MountPoint: path,
			TotalGB:    float64(total) / 1024 / 1024 / 1024,
			UsedGB:     float64(used) / 1024 / 1024 / 1024,
			Usage:      usage,
		}

		result.Mounts = append(result.Mounts, mount)

		if usage > float64(threshold) {
			result.Alert = true
		}
	}

	return result, nil
}
