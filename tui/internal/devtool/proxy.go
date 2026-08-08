package devtool

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// ProxyDir is the mq-proxy module directory, relative to the tui module —
// i.e. this assumes the caller's working directory is tui/, matching every
// other Taskfile.yml entry that runs `go run` from there.
const ProxyDir = "../mq-proxy"

const (
	proxyPIDFile  = "devtool.pid"
	proxyLogFile  = "devtool.log"
	proxyReadyURL = "http://localhost:8080/api/queues"
)

// ProxyRunning reports whether an mq-proxy instance is currently answering
// HTTP requests at its default address.
func ProxyRunning() bool {
	return waitHTTP(context.Background(), proxyReadyURL, 500*time.Millisecond) == nil
}

// StartProxy builds (if needed) and launches mq-proxy as a background
// process from ProxyDir, waiting up to timeout for it to start answering
// HTTP requests. The process ID is written to ProxyDir/devtool.pid so
// StopProxy can find it later.
func StartProxy(ctx context.Context, timeout time.Duration) error {
	if ProxyRunning() {
		return fmt.Errorf("mq-proxy is already responding at %s (stop it first with stop-proxy)", proxyReadyURL)
	}

	gradlew := "./gradlew"
	if runtime.GOOS == "windows" {
		gradlew = "gradlew.bat"
	}
	build := exec.CommandContext(ctx, gradlew, "bootJar")
	build.Dir = ProxyDir
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("%s bootJar: %w", gradlew, err)
	}

	logPath := filepath.Join(ProxyDir, proxyLogFile)
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer logFile.Close()

	run := exec.Command(javaBinary(), "-jar", "build/libs/mq-proxy.jar")
	run.Dir = ProxyDir
	run.Stdout = logFile
	run.Stderr = logFile
	if err := run.Start(); err != nil {
		return fmt.Errorf("start mq-proxy: %w", err)
	}

	pidPath := filepath.Join(ProxyDir, proxyPIDFile)
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(run.Process.Pid)), 0o644); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}

	if err := waitHTTP(ctx, proxyReadyURL, timeout); err != nil {
		// Don't leave a pid file pointing at a process that never became
		// ready (crashed or hung) — it would confuse the next start/stop.
		run.Process.Kill()
		os.Remove(pidPath)
		return fmt.Errorf("mq-proxy did not become ready: %w (see %s)", err, logPath)
	}
	return nil
}

// StopProxy stops the mq-proxy instance started by StartProxy, if any.
func StopProxy() error {
	pidPath := filepath.Join(ProxyDir, proxyPIDFile)
	data, err := os.ReadFile(pidPath)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("no mq-proxy pid file found at %s — is it running (started outside devtool)?", pidPath)
	}
	if err != nil {
		return fmt.Errorf("read pid file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("invalid pid file contents %q: %w", data, err)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}
	if err := proc.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("kill process %d: %w", pid, err)
	}
	os.Remove(pidPath)
	os.Remove(filepath.Join(ProxyDir, proxyLogFile))
	return nil
}

// javaBinary resolves which java executable to launch mq-proxy with.
// exec.Command("java", ...) would resolve "java" from PATH regardless of
// JAVA_HOME, which silently picks the wrong JDK on machines (like this one,
// via sdkman) where PATH's default java isn't the version mq-proxy's
// toolchain requires. Prefer $JAVA_HOME/bin/java when set.
func javaBinary() string {
	home := os.Getenv("JAVA_HOME")
	if home == "" {
		return "java"
	}
	name := "java"
	if runtime.GOOS == "windows" {
		name = "java.exe"
	}
	return filepath.Join(home, "bin", name)
}

// waitHTTP polls url until it returns any HTTP response (even an error
// status — that still means something is listening) or timeout elapses.
func waitHTTP(ctx context.Context, url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for %s", timeout, url)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
}
