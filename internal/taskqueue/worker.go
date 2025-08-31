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
	asynqClient = asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	asynqMux.HandleFunc("evaluate_rule", evaluateAndExecuteTask)
	asynqSrv = asynq.NewServer(asynq.RedisClientOpt{Addr: redisAddr}, asynq.Config{Concurrency: 10})
	if err := asynqSrv.Run(asynqMux); err != nil {
		log.Fatalf("Failed to start workers: %v", err)
	}
}

// StopWorkers stops workers
func StopWorkers() {
	asynqSrv.Stop()
	asynqClient.Close()
}

// Expand with more handlers, queues (e.g., priority)
