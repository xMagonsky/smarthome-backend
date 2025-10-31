package taskqueue

import (
	"log"

	"github.com/hibiken/asynq"
)

var (
	asynqClient *asynq.Client
	asynqMux    = asynq.NewServeMux()
	asynqSrv    *asynq.Server
)

// StartWorkers starts Asynq workers
func StartWorkers(redisAddr string) {
	log.Printf("TASKQUEUE: Starting Asynq workers with Redis at %s", redisAddr)
	asynqClient = asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	asynqMux.HandleFunc("device_update", processDeviceUpdateTask)
	asynqMux.HandleFunc("evaluate_rule", evaluateAndExecuteTask)
	asynqSrv = asynq.NewServer(asynq.RedisClientOpt{Addr: redisAddr}, asynq.Config{Concurrency: 10})
	log.Printf("TASKQUEUE: Workers started, waiting for tasks...")
	if err := asynqSrv.Run(asynqMux); err != nil {
		log.Fatalf("TASKQUEUE: Failed to start workers: %v", err)
	}
}

// StopWorkers stops workers
func StopWorkers() {
	log.Printf("TASKQUEUE: Stopping workers...")
	asynqSrv.Stop()
	asynqClient.Close()
	log.Printf("TASKQUEUE: Workers stopped")
}

// Expand with more handlers, queues (e.g., priority)
