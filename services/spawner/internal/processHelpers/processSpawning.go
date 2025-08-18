package processHelpers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"idia-astro/go-carta/pkg/grpc"
	"idia-astro/go-carta/pkg/shared"
)

// package-scope regex and parser for worker readiness log lines
var listenRe = regexp.MustCompile(`server listening at .*:(\d+)`)

func parsePortFromLine(line string) (int, bool) {
	m := listenRe.FindStringSubmatch(line)
	if len(m) != 2 {
		return 0, false
	}
	p, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return p, true
}

// SpawnWorker starts a new worker process and waits until the worker logs that
// it is listening ("server listening at ..."). The worker is started with
// -port=0 so the OS selects a free port, and the detected port from the log is
// returned.
func SpawnWorker(ctx context.Context, workerPath string, timeoutDuration time.Duration) (*exec.Cmd, int, error) {
	cmd := exec.CommandContext(ctx, workerPath, "-port=0")

	// Capture stdout/stderr so we can watch for the readiness log while still
	// forwarding output to the parent process' stdio.
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start the worker process.
	if err := cmd.Start(); err != nil {
		return nil, 0, fmt.Errorf("failed to start worker: %w", err)
	}

	// Channel to signal readiness once the expected log line is observed
	// (carries the detected port).
	readyCh := make(chan int, 1)

	// TODO: I need to go over this code a bit more
	// Helper to scan a pipe, forward lines, and watch for readiness.
	watch := func(r io.Reader, w io.Writer) {
		s := bufio.NewScanner(r)
		for s.Scan() {
			line := s.Text()
			// Forward the line to the appropriate writer.
			_, err := fmt.Fprintln(w, line)
			if err != nil {
				return
			}
			// Detect readiness: parse port from log line.
			if p, ok := parsePortFromLine(line); ok {
				fmt.Printf("Detected worker port from log: %d\n", p)
				// Send detected port if not already sent.
				select {
				case readyCh <- p:
				default:
				}
			}
		}
	}

	// Start scanning goroutines.
	go watch(stdoutPipe, os.Stdout)
	go watch(stderrPipe, os.Stderr)

	// Wait for readiness or timeout; kill the worker on failure.
	ctxReady, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()
	select {
	case p := <-readyCh:
		return cmd, p, nil
	case <-ctxReady.Done():
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, 0, fmt.Errorf("worker did not become ready in time: %w", ctxReady.Err())
	}
}

func TestWorker(ctx context.Context, port int, timeoutDuration time.Duration) error {
	addr := fmt.Sprintf("localhost:%d", port)
	// Create a client connection (non-blocking)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("could not connect to worker at %s: %w", addr, err)
	}
	defer helpers.CloseOrLog(conn)

	client := cartaProto.NewFileInfoServiceClient(conn)
	rpcCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	r, err := client.CheckStatus(rpcCtx, &cartaProto.Empty{})
	if err != nil {
		return fmt.Errorf("fileInfoService CheckStatus failed: %w", err)
	}

	fmt.Printf("Worker FileInfoService responded (instanceID: %s): %s\n", r.InstanceId, r.StatusMessage)
	return nil
}
