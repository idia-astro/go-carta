package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/google/uuid"

	"idia-astro/go-carta/services/spawner/internal"
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

	app := fiber.New()

	// Start a new worker
	app.Post("/", func(c *fiber.Ctx) error {
		startTime := time.Now()
		cmd, port, err := processUtils.SpawnWorker(ctx, *workerProcess, time.Duration(*timeout)*time.Second)
		spawnerDuration := time.Since(startTime)
		if err != nil {
			log.Errorf("Error spawning worker on free port: %v\n", err)
			return c.Status(fiber.StatusInternalServerError).JSON(&fiber.Map{"msg": "Error spawning worker"})
		}

		startTime = time.Now()
		err = processUtils.TestWorker(ctx, port, 2*time.Second)
		testWorkerDuration := time.Since(startTime)
		if err != nil {
			log.Errorf("Error connecting to worker: %v\n", err)
			c.Status(fiber.StatusInternalServerError)
			err := cmd.Process.Kill()
			if err != nil {
				log.Errorf("Error killing worker: %v\n", err)
			}
			return c.Status(fiber.StatusInternalServerError).JSON(&fiber.Map{"msg": "Error connecting to worker"})
		}
		log.Infof("Started worker on port: %d\n", port)
		workerId := uuid.New()
		workerMap[workerId.String()] = &WorkerInfo{
			Process: cmd,
			Port:    port,
		}
		// Add timing metrics
		c.Set("Server-Timing", fmt.Sprintf("spawn-time;dur=%.2f, check-time;dur=%.2f", spawnerDuration.Seconds()*1000.0, testWorkerDuration.Seconds()*1000.0))
		return c.JSON(&fiber.Map{"port": port, "workerId": workerId.String()})
	})

	// List all workers
	app.Get("/workers", func(c *fiber.Ctx) error {
		// return empty array if no workers
		if len(workerMap) == 0 {
			return c.JSON([]string{})
		}

		var workerIds []string
		for key := range workerMap {
			workerIds = append(workerIds, key)
		}
		return c.JSON(workerIds)
	})

	// List details of a specific worker
	app.Get("/worker/:id", func(c *fiber.Ctx) error {
		workerId := c.Params("id")
		info := workerMap[workerId]
		if info == nil {
			return c.Status(fiber.StatusNotFound).JSON(&fiber.Map{"msg": "Worker not found"})
		}

		alive := info.Process.ProcessState == nil

		output := fiber.Map{
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
			err := processUtils.TestWorker(ctx, info.Port, 1*time.Second)
			elapsed := time.Since(start)
			if err != nil {
				log.Errorf("Error connecting to worker: %v\n", err)
				isReachable = false
			} else {
				c.Set("Server-Timing", fmt.Sprintf("check-time;dur=%.2f", elapsed.Seconds()*1000.0))
			}
			output["isReachable"] = isReachable
		}

		return c.JSON(output)
	})

	// Stop a specific worker
	app.Delete("/worker/:id", func(c *fiber.Ctx) error {
		workerId := c.Params("id")
		info := workerMap[workerId]
		if info == nil {
			return c.Status(fiber.StatusNotFound).JSON(&fiber.Map{"msg": "Worker not found"})
		}

		start := time.Now()
		err := info.Process.Process.Kill()
		elapsed := time.Since(start)

		if err != nil {
			log.Errorf("Error stopping worker: %v\n", err)
			return c.Status(fiber.StatusInternalServerError).JSON(&fiber.Map{"msg": "Error stopping worker"})
		}
		delete(workerMap, workerId)

		c.Set("Server-Timing", fmt.Sprintf("stop-time;dur=%.2f", elapsed.Seconds()*1000.0))
		return c.JSON(&fiber.Map{"msg": "Worker stopped"})
	})

	log.Fatal(app.Listen(fmt.Sprintf(":%d", *port)))
}
