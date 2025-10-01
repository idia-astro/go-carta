package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	sharedDefs "idia-astro/go-carta/pkg/shared/defs"
	"idia-astro/go-carta/services/spawner/internal/httpHelpers"
	"idia-astro/go-carta/services/spawner/internal/processHelpers"
)

var (
	workerProcess = flag.String("workerProcess", "build/worker", "Path to worker binary")
	port          = flag.Int("port", 8080, "HTTP server port")
	hostname      = flag.String("hostname", "", "Hostname to listen on")
	timeout       = flag.Int("timeout", 5, "Spawn timeout in seconds")
)

type WorkerInfo struct {
	Cmd      *exec.Cmd
	Port     int
	Username string
}

func main() {
	flag.Parse()

	id := uuid.New()
	log.Printf("Started spawner with UUID: %s\n", id.String())
	// Global context that cancels all spawned processes on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	workerMap := make(map[string]*WorkerInfo)

	r := chi.NewRouter()

	// Start a new worker
	r.Post("/", func(w http.ResponseWriter, r *http.Request) {
		var body sharedDefs.WorkerSpawnBody
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// TODO: Actual logic relating to spawning a worker as the specified user

		startTime := time.Now()
		cmd, port, err := processHelpers.SpawnWorker(ctx, *workerProcess, time.Duration(*timeout)*time.Second)
		spawnerDuration := time.Since(startTime)
		if err != nil {
			log.Printf("Error spawning worker on free port: %v\n", err)
			httpHelpers.WriteError(w, http.StatusInternalServerError, "Error spawning worker")
			return
		}

		startTime = time.Now()
		err = processHelpers.TestWorker(ctx, port, 2*time.Second)
		testWorkerDuration := time.Since(startTime)
		if err != nil {
			log.Printf("Error connecting to worker: %v\n", err)
			err := cmd.Process.Kill()
			if err != nil {
				log.Printf("Error killing worker: %v\n", err)
			}
			httpHelpers.WriteError(w, http.StatusInternalServerError, "Error connecting to worker")
			return
		}
		log.Printf("Started worker on port: %d\n", port)
		workerId := uuid.New()
		workerMap[workerId.String()] = &WorkerInfo{
			Username: body.Username,
			Cmd:      cmd,
			Port:     port,
		}
		httpHelpers.WriteTimings(w, httpHelpers.Timings{"spawn-time": spawnerDuration, "check-time": testWorkerDuration})

		workerHostname := *hostname
		if workerHostname == "" {
			workerHostname = "localhost"
		}

		httpHelpers.WriteOutput(w, map[string]any{"port": port, "address": workerHostname, "workerId": workerId.String()})
	})

	// List all workers
	r.Get("/workers", func(w http.ResponseWriter, r *http.Request) {
		// return empty array if no workers
		if len(workerMap) == 0 {
			httpHelpers.WriteOutput(w, []string{})
			return
		}

		var workerIds []sharedDefs.WorkerListItem
		for key, val := range workerMap {
			workerIds = append(workerIds, sharedDefs.WorkerListItem{
				WorkerId:  key,
				ProcessId: val.Cmd.Process.Pid,
				Username:  val.Username,
				UserId:    val.Cmd.SysProcAttr.Credential.Uid,
			})
		}
		httpHelpers.WriteOutput(w, workerIds)
	})

	// Get details of a specific worker
	r.Get("/worker/{id}", func(w http.ResponseWriter, r *http.Request) {
		workerId := chi.URLParam(r, "id")
		info := workerMap[workerId]
		if info == nil {
			httpHelpers.WriteError(w, http.StatusNotFound, "Worker not found")
			return
		}

		workerHostname := *hostname
		if workerHostname == "" {
			workerHostname = "localhost"
		}

		alive := info.Cmd.ProcessState == nil

		output := sharedDefs.WorkerStatus{
			WorkerInfo: sharedDefs.WorkerInfo{
				Port:      info.Port,
				Address:   workerHostname,
				WorkerId:  workerId,
				UserId:    info.Cmd.SysProcAttr.Credential.Uid,
				ProcessId: info.Cmd.Process.Pid,
			},
			Alive: alive,
		}

		if !alive {
			output.ExitedCleanly = info.Cmd.ProcessState != nil && info.Cmd.ProcessState.Success()
		} else {
			isReachable := true
			start := time.Now()
			err := processHelpers.TestWorker(ctx, info.Port, 1*time.Second)
			elapsed := time.Since(start)
			if err != nil {
				log.Printf("Error connecting to worker: %v\n", err)
				isReachable = false
			} else {
				httpHelpers.WriteTimings(w, httpHelpers.Timings{"check-time": elapsed})
			}
			output.IsReachable = isReachable
		}

		httpHelpers.WriteOutput(w, output)
	})

	// Stop a specific worker
	r.Delete("/worker/{id}", func(w http.ResponseWriter, r *http.Request) {
		workerId := chi.URLParam(r, "id")
		info := workerMap[workerId]
		if info == nil {
			httpHelpers.WriteError(w, http.StatusNotFound, "Worker not found")
			return
		}

		start := time.Now()
		err := info.Cmd.Process.Kill()
		elapsed := time.Since(start)

		if err != nil {
			log.Printf("Error stopping worker: %v\n", err)
			httpHelpers.WriteError(w, http.StatusInternalServerError, "Error stopping worker")
			return
		}
		delete(workerMap, workerId)

		httpHelpers.WriteTimings(w, httpHelpers.Timings{"stop-time": elapsed})
		httpHelpers.WriteOutput(w, map[string]any{"msg": "Worker stopped"})
	})

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", *hostname, *port),
		Handler: r,
	}
	// Run server in background
	go func() {
		log.Printf("Starting spawner on %s:%d\n", *hostname, *port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	// Wait for interrupt
	<-ctx.Done()
	log.Println("Signal received, shutting down...")

	for id, w := range workerMap {
		// If the worker is not running, skip it
		if w.Cmd != nil && w.Cmd.Process != nil {
			// First try a graceful shutdown
			err := w.Cmd.Process.Signal(syscall.SIGTERM)
			if err != nil {
				log.Printf("Error sending SIGTERM to process: %v\n", err)
				continue
			}

			// Wait for it to exit
			done := make(chan error, 1)
			go func() { done <- w.Cmd.Wait() }()

			select {
			case err := <-done:
				log.Printf("process exited: %v\n", err)
			case <-time.After(5 * time.Second):
				log.Println("timeout, force killing")
				if err := w.Cmd.Process.Kill(); err != nil {
					log.Printf("Error force killing process: %v\n", err)
				}
				<-done // wait again to reap zombie
			}
		}
		delete(workerMap, id)
	}

	// Shutdown the HTTP server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server Shutdown error: %v", err)
	} else {
		log.Println("HTTP server shut down gracefully")
	}
	cancel()

	// Wait a moment to ensure all logs are printed before exiting
	time.Sleep(1 * time.Second)

	log.Println("Spawner exited gracefully")
}
