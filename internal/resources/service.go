package resources

import (
	"encoding/json"
	"fmt"
	"time"

	"nmsappsrv/pkg/logger"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

// Service defines the business-logic contract for resource monitoring
type Service interface {
	GetCpuAndMemUsage() ResourcesVO
	GetTableStatus() ([]TableStatusVO, error)
	GetDiskUsage() []DiskUsageVO
	GetThreshold() (*ThresholdConfig, error)
	UpdateThreshold(cfg *ThresholdConfig) error
}

// service is the concrete implementation of Service
type service struct {
	repo Repository
}

// NewService creates a new Service
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// GetCpuAndMemUsage returns current CPU and memory usage
func (s *service) GetCpuAndMemUsage() ResourcesVO {
	cpuUsage := 0.0
	memUsage := 0.0

	percent, err := cpu.Percent(1*time.Second, false)
	if err != nil {
		logger.Warnf("failed to sample CPU usage: %v", err)
	} else if len(percent) > 0 {
		cpuUsage = percent[0]
	}

	v, err := mem.VirtualMemory()
	if err != nil {
		logger.Warnf("failed to sample memory usage: %v", err)
	} else {
		memUsage = v.UsedPercent
	}

	return ResourcesVO{
		CPU:       cpuUsage,
		Mem:       memUsage,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// GetTableStatus returns MySQL table sizes
func (s *service) GetTableStatus() ([]TableStatusVO, error) {
	return s.repo.GetTableStatus()
}

// GetDiskUsage returns disk partition usage
func (s *service) GetDiskUsage() []DiskUsageVO {
	result := []DiskUsageVO{}

	partitions, err := disk.Partitions(false)
	if err != nil {
		logger.Warnf("failed to list disk partitions: %v", err)
		return result
	}

	for _, p := range partitions {
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			logger.Warnf("failed to get disk usage for %s: %v", p.Mountpoint, err)
			continue
		}
		result = append(result, DiskUsageVO{
			Filesystem: p.Device,
			Size:       formatBytes(usage.Total),
			Used:       formatBytes(usage.Used),
			Avail:      formatBytes(usage.Free),
			UsePercent: fmt.Sprintf("%.1f%%", usage.UsedPercent),
			MountPoint: p.Mountpoint,
		})
	}

	return result
}

// formatBytes converts bytes to a human-readable string (e.g. "1.5 GB")
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), []string{"KB", "MB", "GB", "TB", "PB"}[exp])
}

// GetThreshold returns alarm threshold configuration
func (s *service) GetThreshold() (*ThresholdConfig, error) {
	value, err := s.repo.GetSystemConfig("sysThreshold")
	if err != nil {
		return nil, err
	}

	defaults := &ThresholdConfig{
		CPU:       60,
		Mem:       60,
		Disk:      80,
		DiskClear: 80,
		Table:     5.0,
	}

	if value == "" {
		return defaults, nil
	}

	var cfg ThresholdConfig
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		return defaults, nil
	}
	return &cfg, nil
}

// UpdateThreshold updates alarm threshold configuration
func (s *service) UpdateThreshold(cfg *ThresholdConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.repo.SaveSystemConfig("sysThreshold", string(data))
}

// newService creates a Service backed by the given mock Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}
