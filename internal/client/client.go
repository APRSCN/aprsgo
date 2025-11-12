package client

import (
	"time"

	"github.com/APRSCN/aprsgo/internal/logger"
)

// InitClient inits client daemon
func InitClient() {
	//ch2, _ := uplink.Stream.Subscribe()
	//go worker(2, ch2)

	logger.L.Debug("Client daemon initialized")
}

func worker(id int, dataCh <-chan string) {
	for data := range dataCh {
		println("Worker", id, "received:", data)
		time.Sleep(time.Millisecond * 10)
	}
	println("Worker", id, "stopped")
}
