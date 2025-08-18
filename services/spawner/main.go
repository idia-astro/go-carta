package main

import (
	"context"
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

	"idia-astro/go-carta/services/spawner/internal/httpHelpers"
	"idia-astro/go-carta/services/spawner/internal/processHelpers"
)

var (
	workerProcess = flag.String("workerProcess", "build/worker", "Path to worker binary")
	port          = flag.Int("port", 8080, "HTTP server port")
	timeout       = flag.Int("timeout", 5, "Spawn timeout in seconds")
)

type WorkerInfo struct {
	Process *exec.Cmd
	Port    int
}

func main() {
	id := uuid.New()
	fmt.Printf("Started spawner with UUID: %s\n", id.String())
	// Global context that cancels all spawned processes on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	workerMap := make(map[string]*WorkerInfo)

	r := chi.NewRouter()

	// Start a new worker
	r.Post("/", func(w http.ResponseWriter, r *http.Request) {
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
			Process: cmd,
			Port:    port,
		}
		// Add timing metrics
		w.Header().Set("Server-Timing", fmt.Sprintf("spawn-time;dur=%.2f, check-time;dur=%.2f", spawnerDuration.Seconds()*1000.0, testWorkerDuration.Seconds()*1000.0))
		httpHelpers.WriteOutput(w, map[string]any{"port": port, "workerId": workerId.String()})
	})

	// List all workers
	r.Get("/workers", func(w http.ResponseWriter, r *http.Request) {
		// return empty array if no workers
		if len(workerMap) == 0 {
			httpHelpers.WriteOutput(w, []string{})
			return
		}

		var workerIds []string
		for key := range workerMap {
			workerIds = append(workerIds, key)
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

		alive := info.Process.ProcessState == nil

		output := map[string]any{
			"port":     info.Port,
			"workerId": workerId,
			"pid":      info.Process.Process.Pid,
			"alive":    alive,
		}

		if !alive {
			output["exitedCleanly"] = info.Process.ProcessState != nil && info.Process.ProcessState.Success()
		} else {
			isReachable := true
			start := time.Now()
			err := processHelpers.TestWorker(ctx, info.Port, 1*time.Second)
			elapsed := time.Since(start)
			if err != nil {
				log.Printf("Error connecting to worker: %v\n", err)
				isReachable = false
			} else {
				w.Header().Set("Server-Timing", fmt.Sprintf("check-time;dur=%.2f", elapsed.Seconds()*1000.0))
			}
			output["isReachable"] = isReachable
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
		err := info.Process.Process.Kill()
		elapsed := time.Since(start)

		if err != nil {
			log.Printf("Error stopping worker: %v\n", err)
			httpHelpers.WriteError(w, http.StatusInternalServerError, "Error stopping worker")
			return
		}
		delete(workerMap, workerId)

		w.Header().Set("Server-Timing", fmt.Sprintf("stop-time;dur=%.2f", elapsed.Seconds()*1000.0))
		httpHelpers.WriteOutput(w, map[string]any{"msg": "Worker stopped"})
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: r,
	}

	log.Fatal(server.ListenAndServe())
}
