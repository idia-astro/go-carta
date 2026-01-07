package processHelpers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	helpers "github.com/idia-astro/go-carta/pkg/shared"
)

// package-scope regex and parser for worker readiness log lines
var listenRe = regexp.MustCompile(`Listening on port (\d+) with top level folder`)

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
func SpawnWorker(ctx context.Context, workerPath string, timeoutDuration time.Duration, baseFolder string) (*exec.Cmd, int, error) {
	args := []string{"--debug_no_auth"}
	args = append(args, "--no_frontend")
	args = append(args, "--verbosity", "5")
	args = append(args, "--exit_timeout", "10")
	args = append(args, "--initial_timeout", "20")
	args = append(args, "--idle_timeout", "300")
	if baseFolder != "" {
		args = append(args, "--base", baseFolder)
	}

	log.Printf("\n\n ***** Spawning worker process: %s %v\n\nn", workerPath, args)

	cmd := exec.CommandContext(ctx, workerPath, args...)

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

	log.Println("[carta-spawn] ***** Worker process started, waiting for readiness...")

	// TODO: I need to go over this code a bit more
	// Helper to scan a pipe, forward lines, and watch for readiness.
	watch := func(r io.Reader, w io.Writer) {
		s := bufio.NewScanner(r)
		for s.Scan() {
			line := s.Text()
			// Forward the line to the appropriate writer.
			//log.Printf("[carta-spawn] %s\n", line)
			_, err := fmt.Fprintln(w, line)
			if err != nil {
				return
			}
			// Detect readiness: parse port from log line.
			log.Printf("[carta-spawn] Scanning line for port info: %s\n\n", line)
			if p, ok := parsePortFromLine(line); ok {
				log.Printf("[carta-spawn] +++++++++ Detected worker port from log: %d\n\n", p)
				// Send detected port if not already sent.
				select {
				case readyCh <- p:
				default:
				}
			}
			log.Printf("[carta-spawn] Finished scanning line: %s\n", line)
		}
	}

	log.Println("[carta-spawn] ***** Starting to watch worker stdout/stderr for readiness...")

	// Start scanning goroutines.
	go watch(stdoutPipe, os.Stdout)
	go watch(stderrPipe, os.Stderr)

	log.Println("[carta-spawn] ***** Watching worker output for readiness...")

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
	addr := fmt.Sprintf("ws://localhost:%d", port)

	rpcCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()
	// Connect to the worker websocket
	conn, _, err := websocket.DefaultDialer.DialContext(rpcCtx, addr, nil)
	if err != nil {
		return err
	}
	defer helpers.CloseOrLog(conn)

	// Send a PING text message and wait for a PONG
	err = conn.WriteMessage(websocket.TextMessage, []byte("PING"))
	if err != nil {
		return err
	}
	messageType, message, err := conn.ReadMessage()
	if err != nil {
		return err
	}
	if messageType != websocket.TextMessage {
		return fmt.Errorf("expected text message, got %d", messageType)
	}
	if string(message) != "PONG" {
		return fmt.Errorf("expected PONG, got %s", string(message))
	}

	return nil
}
