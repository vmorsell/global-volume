package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/vmorsell/global-volume/internal/volume"
)

func main() {
	fmt.Println("🌐 Global Volume Sync")
	fmt.Println("────────────────────────────")

	listener := volume.NewListener()
	volCh, err := listener.Listen()
	if err != nil {
		fmt.Printf("[ERROR] Failed to start volume listener: %v\n", err)
		return
	}

	currentVolume, err := listener.GetCurrentVolume()
	if err != nil {
		fmt.Printf("Failed to get current volume: %v\n", err)
		return
	}
	fmt.Printf("[INFO] Local volume: %d%%\n", currentVolume)

	// handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case vol := <-volCh:
			fmt.Printf("[INFO] Local volume: %d%%\n", vol)
		case <-sigCh:
			fmt.Println("\n[INFO] Shutting down...")
			return
		}
	}
}
