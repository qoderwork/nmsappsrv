package tcpdump

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"nmsappsrv/pkg/logger"
)

const (
	// defaultCaptureDir mirrors Java's /home/files/tcpdump_file — the shared
	// volume all (api/comm/core) containers wrote captures into and the api
	// container listed/served from.
	defaultCaptureDir = "/home/files/tcpdump_file"
	// defaultInterface mirrors Java's hardcoded "eth0". In Java each
	// api/comm/core container captured on its own eth0 (the device-facing NIC
	// for comm/core). nmsappsrv is a single host process, so its eth0 is the
	// device-facing NIC — behaviourally identical to Java.
	defaultInterface = "eth0"
	// ipInfoPath mirrors Java's /home/ip.info used by listNetworkCards.
	ipInfoPath = "/home/ip.info"
)

// Service encapsulates tcpdump business logic: starting captures (fire-and-forget,
// matching Java's @Async TCPDumpHelper) and managing the capture-file lifecycle
// (list/download/delete) — all file-based, exactly like Java (no task table).
type Service struct {
	captureDir string
	iface      string
}

// NewService creates a Service. db is accepted for signature compatibility with
// the rest of the app but is unused: tcpdump is purely file-based in nmsappsrv,
// as it was in nms-serv.
func NewService(db interface{}) *Service {
	dir := defaultCaptureDir
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Errorf("tcpdump: failed to create capture dir %s: %v", dir, err)
	}
	iface := os.Getenv("TCPDUMP_IFACE")
	if iface == "" {
		iface = defaultInterface
	}
	return &Service{captureDir: dir, iface: iface}
}

// DoCapture launches a tcpdump process in the background (fire-and-forget, like
// Java's @Async TCPDumpHelper.doTcpdump). The capture runs for `duration` seconds
// and writes a .pcap file named "<container>_<start>_<end>.pcap" into the capture
// directory — byte-for-byte the same filename scheme and command Java used.
func (s *Service) DoCapture(req *DoCaptureRequest) error {
	if strings.TrimSpace(req.Container) == "" {
		return fmt.Errorf("container must not be empty")
	}
	if req.Duration < 0 {
		return fmt.Errorf("duration must not be negative")
	}

	start := time.Now()
	end := start.Add(time.Duration(req.Duration) * time.Second)
	fileName := buildCaptureFileName(req.Container, start, end)
	filePath := filepath.Join(s.captureDir, fileName)

	// Mirror Java's command array exactly:
	//   /usr/bin/timeout <duration> /usr/bin/tcpdump -i eth0 -w <file>
	args := []string{
		strconv.Itoa(req.Duration),
		"/usr/bin/tcpdump",
		"-i", s.iface,
		"-w", filePath,
	}
	cmd := exec.Command("/usr/bin/timeout", args...)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("tcpdump: failed to start capture: %w", err)
	}

	// Fire-and-forget: log on completion, no task tracking (matches Java).
	go func() {
		err := cmd.Wait()
		if err != nil {
			exitCode := -1
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
			// 124 = timeout fired (expected), 143 = SIGTERM (expected),
			// 0 = clean exit. Anything else is a real failure.
			if exitCode != 124 && exitCode != 143 && exitCode != 0 {
				logger.Errorf("tcpdump: capture %s exited with code %d: %v", fileName, exitCode, err)
				return
			}
		}
		logger.Infof("tcpdump: capture %s finished", fileName)
	}()

	return nil
}

// ListNetworkCards returns the available network interfaces. Mirrors Java's
// listNetworkCards: read /home/ip.info ("<ip> <name>" per line) when present,
// otherwise enumerate the host's interfaces with a representative IPv4 address.
func (s *Service) ListNetworkCards() ([]NetworkCard, error) {
	if cards, err := readIPInfo(ipInfoPath); err == nil {
		return cards, nil
	}
	return enumerateHostInterfaces()
}

// ListFiles returns all capture files in the capture directory, newest first,
// mirroring Java's listTcpdumpFiles (reads /home/files/tcpdump_file).
func (s *Service) ListFiles() ([]TcpdumpFile, error) {
	entries, err := os.ReadDir(s.captureDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []TcpdumpFile{}, nil
		}
		return nil, err
	}

	files := make([]TcpdumpFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, statErr := e.Info()
		if statErr != nil {
			continue
		}
		files = append(files, TcpdumpFile{
			FileName:   e.Name(),
			ModifyTime: info.ModTime().UnixMilli(),
		})
	}

	// Newest first, matching Java's sort by modifyTime descending.
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModifyTime > files[j].ModifyTime
	})
	return files, nil
}

// DownloadPath resolves a safe on-disk path for the named capture file.
// Mirrors Java's FileUtil.isValidFileName guard against path traversal.
func (s *Service) DownloadPath(name string) (string, error) {
	if !isValidFileName(name) {
		return "", fmt.Errorf("invalid file name: %q", name)
	}
	full := filepath.Join(s.captureDir, name)
	if _, err := os.Stat(full); err != nil {
		return "", fmt.Errorf("capture file not found: %s", name)
	}
	return full, nil
}

// DeleteFile removes a single capture file from disk.
func (s *Service) DeleteFile(name string) error {
	if !isValidFileName(name) {
		return fmt.Errorf("invalid file name: %q", name)
	}
	full := filepath.Join(s.captureDir, name)
	if _, err := os.Stat(full); err != nil {
		return fmt.Errorf("capture file not found: %s", name)
	}
	return os.Remove(full)
}

// BatchDelete removes multiple capture files, skipping any that are missing or
// have invalid names (per-file failure does not abort the rest).
func (s *Service) BatchDelete(names []string) error {
	var firstErr error
	for _, name := range names {
		if !isValidFileName(name) {
			if firstErr == nil {
				firstErr = fmt.Errorf("invalid file name: %q", name)
			}
			continue
		}
		full := filepath.Join(s.captureDir, name)
		if _, err := os.Stat(full); err != nil {
			continue // missing — skip
		}
		if err := os.Remove(full); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// buildCaptureFileName mirrors Java's TCPDumpHelper filename scheme:
// "<container>_<yyyyMMddHHmmss>_<yyyyMMddHHmmss>.pcap".
func buildCaptureFileName(container string, start, end time.Time) string {
	return fmt.Sprintf("%s_%s_%s.pcap",
		container,
		start.Format("20060102150405"),
		end.Format("20060102150405"),
	)
}

// isValidFileName rejects anything that could escape the capture directory:
// empty names, path separators, or ".." segments.
func isValidFileName(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return false
	}
	// filepath.Base strips a leading "./" but keeps a bare name; ensure the
	// cleaned base exactly equals the input (no directory components).
	return filepath.Base(name) == name
}

// readIPInfo parses a "<ip> <name>" per-line file (Java's /home/ip.info).
func readIPInfo(path string) ([]NetworkCard, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cards []NetworkCard
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			cards = append(cards, NetworkCard{IP: fields[0], Name: fields[1]})
		}
	}
	return cards, nil
}

// enumerateHostInterfaces returns each host network interface with its first
// non-loopback IPv4 address (used when /home/ip.info is absent).
func enumerateHostInterfaces() ([]NetworkCard, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var cards []NetworkCard
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, addrErr := iface.Addrs()
		if addrErr != nil {
			continue
		}
		ip := ""
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				ip = ipNet.IP.String()
				break
			}
		}
		cards = append(cards, NetworkCard{IP: ip, Name: iface.Name})
	}
	return cards, nil
}
