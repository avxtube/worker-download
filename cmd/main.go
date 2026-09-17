package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"worker-download/internal/config"
	"worker-download/internal/core/utils"
	"worker-download/internal/dashboard"
	"worker-download/internal/db/database"
	"worker-download/internal/download"
	"worker-download/internal/queue"
)

// version ถูกฝังตอน build โดย GitHub Actions: -ldflags="-X main.version=v1.x.x"
var version = "dev"

func main() {
	log.SetOutput(os.Stdout)
	config.Load()
	config.AppConfig.WorkerVersion = version
	workerID := utils.GenerateWorkerID()
	log.Printf("🚀 Starting Worker Download %s [Worker: %s]", version, workerID)

	// Runtime artifacts stay beside the installed binary. Locally that means
	// .build/work and .build/.log; Linux releases use /opt/worker-download/work
	// and /opt/worker-download/.log.
	for _, dir := range []string{config.AppConfig.WorkDir, config.AppConfig.LogDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("❌ Failed to create runtime directory %s: %v", dir, err)
			os.Exit(1)
		}
	}

	// ── MongoDB ───────────────────────────────────────────────
	if err := database.Connect(); err != nil {
		log.Printf("❌ Failed to connect to MongoDB: %v", err)
		time.Sleep(5 * time.Second) // ให้ log ถูก flush / กัน restart-loop รัวๆ
		os.Exit(1)
	}
	defer database.Disconnect()

	// ── Heartbeat ─────────────────────────────────────────────
	// ctx ยกเลิกเมื่อโดน SIGINT/SIGTERM → heartbeat mark ตัวเอง offline ก่อนจบ
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logDir := config.AppConfig.LogDir
	utils.CleanOldLogs(logDir)
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				utils.CleanOldLogs(logDir)
			}
		}
	}()

	hbDone := make(chan struct{})
	go func() {
		defer close(hbDone)
		queue.StartHeartbeat(ctx, workerID)
	}()

	if dashboard.ShouldStart(workerID) {
		go dashboard.Start(ctx, config.AppConfig.DashboardPort, workerID, config.AppConfig.WorkDir, logDir)
	} else {
		log.Printf("📺 Download monitor owned by worker @1 (this worker: %s)", workerID)
	}

	// ── Job loop (blocking จนโดน SIGINT/SIGTERM) ──────────────
	// shutdown ระหว่างทำงาน → loop จะ Release งานคืนคิวให้เอง
	queue.RunLoop(ctx, workerID, download.Run)

	log.Println("🛑 Shutting down...")

	// รอ heartbeat ปิดตัว (mark offline) ให้เสร็จก่อน disconnect DB
	select {
	case <-hbDone:
	case <-time.After(10 * time.Second):
		log.Println("⚠️ Heartbeat shutdown timed out")
	}
}
