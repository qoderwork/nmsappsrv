package tcpdump

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

const (
	defaultCaptureDir = "./data/tcpdump"
	defaultInterface  = "eth0"
)

// Service encapsulates tcpdump business logic: starting/stopping captures and
// managing the lifecycle of capture tasks.
type Service struct {
	db         *gorm.DB
	captureDir string
	mu         sync.Mutex
	// activeTasks tracks running OS processes keyed by task ID so we can kill them.
	activeTasks map[int64]*os.Process
}

// NewService creates a Service backed by db. It ensures the capture output
// directory exists.
func NewService(db *gorm.DB) *Service {
	dir := defaultCaptureDir
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Errorf("tcpdump: failed to create capture dir %s: %v", dir, err)
	}
	return &Service{
		db:          db,
		captureDir:  dir,
		activeTasks: make(map[int64]*os.Process),
	}
}

// StartCapture creates a TcpdumpTask record and launches tcpdump in a
// background goroutine. The task runs on the NMS server itself (or can be
// extended to SSH/TR-069 to a remote device).
func (s *Service) StartCapture(req *StartRequest) (int64, error) {
	iface := req.Interface
	if iface == "" {
		iface = defaultInterface
	}

	task := &TcpdumpTask{
		ElementId:   req.ElementId,
		Interface:   iface,
		Filter:      req.Filter,
		Duration:    req.Duration,
		PacketCount: req.PacketCount,
		Status:      StatusRunning,
		CreateTime:  time.Now(),
	}

	if err := s.db.Create(task).Error; err != nil {
		return 0, fmt.Errorf("tcpdump: failed to create task: %w", err)
	}

	// Build the output file path
	startTs := task.CreateTime.Format("20060102150405")
	endTs := task.CreateTime.Add(time.Duration(req.Duration) * time.Second).Format("20060102150405")
	fileName := fmt.Sprintf("tcpdump_%d_%s_%s.pcap", req.ElementId, startTs, endTs)
	task.FilePath = filepath.Join(s.captureDir, fileName)

	// Persist the file path
	s.db.Model(task).Update("file_path", task.FilePath)

	// Launch the actual tcpdump process asynchronously
	go s.runCapture(task, iface)

	return task.Id, nil
}

// runCapture executes the tcpdump command and updates the task when done.
func (s *Service) runCapture(task *TcpdumpTask, iface string) {
	// Build tcpdump command: timeout <duration> tcpdump -i <iface> [-c count] [-w file] [filter]
	args := []string{
		fmt.Sprintf("%d", task.Duration),
		"/usr/bin/tcpdump",
		"-i", iface,
		"-w", task.FilePath,
	}
	if task.PacketCount > 0 {
		args = append(args, "-c", fmt.Sprintf("%d", task.PacketCount))
	}
	if task.Filter != "" {
		args = append(args, task.Filter)
	}

	cmd := exec.Command("/usr/bin/timeout", args...)

	if err := cmd.Start(); err != nil {
		s.failTask(task, fmt.Sprintf("failed to start tcpdump: %v", err))
		return
	}

	// Track the process so StopCapture can kill it
	s.mu.Lock()
	s.activeTasks[task.Id] = cmd.Process
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.activeTasks, task.Id)
		s.mu.Unlock()
	}()

	err := cmd.Wait()

	// timeout exits with 124 when the timer fires; tcpdump may also exit with
	// non-zero when it receives SIGTERM from timeout. Both are expected.
	if err != nil {
		exitCode := -1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		// 124 = timeout killed the process (expected), 143 = SIGTERM (expected)
		if exitCode != 124 && exitCode != 143 && exitCode != 0 {
			s.failTask(task, fmt.Sprintf("tcpdump exited with code %d: %v", exitCode, err))
			return
		}
	}

	// Gather file info
	var fileSize int64
	if info, statErr := os.Stat(task.FilePath); statErr == nil {
		fileSize = info.Size()
	}

	now := time.Now()
	s.db.Model(task).Updates(map[string]interface{}{
		"status":    StatusDone,
		"end_time":  now,
		"file_size": fileSize,
	})

	logger.Infof("tcpdump: task %d completed, file=%s, size=%d bytes", task.Id, task.FilePath, fileSize)
}

// StopCapture kills a running tcpdump process for the given task ID.
func (s *Service) StopCapture(taskId int64) error {
	s.mu.Lock()
	proc, ok := s.activeTasks[taskId]
	s.mu.Unlock()

	if !ok {
		return fmt.Errorf("task %d is not running", taskId)
	}

	if err := proc.Kill(); err != nil {
		return fmt.Errorf("failed to kill tcpdump for task %d: %w", taskId, err)
	}

	logger.Infof("tcpdump: task %d stopped by user", taskId)
	return nil
}

// ListTasks returns all tcpdump tasks, newest first.
func (s *Service) ListTasks() ([]TcpdumpTask, error) {
	var tasks []TcpdumpTask
	if err := s.db.Order("create_time DESC").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// GetTask returns a single task by ID.
func (s *Service) GetTask(id int64) (*TcpdumpTask, error) {
	var task TcpdumpTask
	if err := s.db.First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// DeleteTask removes the task record and its capture file from disk.
func (s *Service) DeleteTask(id int64) error {
	task, err := s.GetTask(id)
	if err != nil {
		return err
	}

	// Refuse to delete a running task
	if task.Status == StatusRunning {
		return fmt.Errorf("cannot delete running task %d; stop it first", id)
	}

	// Remove the file from disk
	if task.FilePath != "" {
		_ = os.Remove(task.FilePath)
	}

	return s.db.Delete(&TcpdumpTask{}, id).Error
}

// GetTaskFilePath returns the file path for a task (used by the download handler).
func (s *Service) GetTaskFilePath(id int64) (string, error) {
	task, err := s.GetTask(id)
	if err != nil {
		return "", err
	}
	if task.Status != StatusDone {
		return "", fmt.Errorf("task %d has not completed yet (status=%d)", id, task.Status)
	}
	if _, statErr := os.Stat(task.FilePath); statErr != nil {
		return "", fmt.Errorf("capture file not found: %s", task.FilePath)
	}
	return task.FilePath, nil
}

// failTask marks a task as failed with the given error message.
func (s *Service) failTask(task *TcpdumpTask, errMsg string) {
	now := time.Now()
	s.db.Model(task).Updates(map[string]interface{}{
		"status":        StatusFailed,
		"end_time":      now,
		"error_message": errMsg,
	})
	logger.Errorf("tcpdump: task %d failed: %s", task.Id, errMsg)
}

// sanitizeFilter performs basic sanitization on the pcap filter expression to
// prevent shell injection. Only alphanumeric, spaces, and common pcap filter
// tokens are allowed.
func sanitizeFilter(filter string) string {
	allowed := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 .:-/()!<>=&|"
	var b strings.Builder
	for _, r := range filter {
		if strings.ContainsRune(allowed, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
