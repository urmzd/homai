package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	DefaultSocketPath = "/tmp/zigbee-skill.sock"
	DefaultPIDPath    = "/tmp/zigbee-skill.pid"
	DefaultLogPath    = "/tmp/zigbee-skill.log"
)

// WritePID writes the current process PID to the file.
func WritePID(path string) error {
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0644)
}

// ReadPID reads and parses the PID file.
func ReadPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// RemovePID removes the PID file.
func RemovePID(path string) {
	_ = os.Remove(path)
}

// IsRunning checks if a daemon is running by reading the PID file and
// sending signal 0 to verify the process exists.
func IsRunning(pidPath string) (bool, int, error) {
	pid, err := ReadPID(pidPath)
	if err != nil {
		return false, 0, err
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, pid, nil
	}
	// Signal 0 checks if process exists without sending a real signal.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, pid, nil
	}
	return true, pid, nil
}

// StopDaemon reads the PID file and sends SIGTERM to the daemon process.
func StopDaemon(pidPath string) error {
	pid, err := ReadPID(pidPath)
	if err != nil {
		return fmt.Errorf("read PID file: %w (is the daemon running?)", err)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM to %d: %w", pid, err)
	}
	// Wait for process to exit (up to 5 seconds).
	for range 50 {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("daemon (pid %d) did not exit after SIGTERM", pid)
}

// Fork re-executes the current binary with --daemon-foreground in the background.
// The child process manages its own log file via lumberjack (size-capped rotation),
// so stdout/stderr are discarded here.
func Fork(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return fmt.Errorf("open %s: %w", os.DevNull, err)
	}
	defer devNull.Close()

	cmd := exec.Command(exe, args...)
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon process: %w", err)
	}

	return nil
}
