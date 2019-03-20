package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	proxyListenPort   = "3128"
	httpsListenPort   = "3129"
	adminDomain       = "rcom.com"
	hostBaseDirectory = "/mnt/idev/docker/rcomproxy"
)

var (
	logger = log.New(os.Stdout, "", 0)
	HostIP = "10.90.7.56"
)

func main() {
	//parse command-line
	var pemPath string
	flag.StringVar(&pemPath, "pem", "", "path to certificate file (fullchain.pem)")
	var keyPath string
	flag.StringVar(&keyPath, "key", "", "path to key file (privkey.pem)")
	var mapPath string
	flag.StringVar(&mapPath, "map", "", "path to map file")

	flag.StringVar(&HostIP, "hostip", HostIP, "Host IP, default: "+HostIP)

	f := flag.Usage
	flag.Usage = func() {
		f()
		logger.Printf("go run *.go proxy --pem=./pem/fullchain.pem --key=./pem/priv.pem")
	}
	var forceRestart bool
	flag.BoolVar(&forceRestart, "f", false, "Force restart all existing proxy servers.")
	flag.Parse()

	var shutdown func(context.Context) error
	logger = log.New(os.Stdout, "", log.LstdFlags)
	mapping := ParseMapFile(mapPath)
	logger.Printf("pem: %v. key: %v. map: %v", pemPath, keyPath, mapPath)

	_, err1 := os.Stat(pemPath)
	_, err2 := os.Stat(keyPath)
	if os.IsNotExist(err1) || os.IsNotExist(err2) {
		logger.Printf("Warning: Cannot find certificate file %s %s. Transfer http only\n", pemPath, keyPath)
		logger.Printf("err1: %v. err2: %v", err1, err2)
		pemPath = ""
		keyPath = ""
	}
	// Start proxy server
	logger.Println("Proxy server is starting...")

	proxyServer := NewProxyServer(proxyListenPort, httpsListenPort, pemPath, keyPath, mapping)
	go proxyServer.Serve()
	shutdown = func(ctx context.Context) error { return proxyServer.Shutdown(ctx) }

	// Gracefully shutdown
	if shutdown != nil {
		done := make(chan bool)
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGSEGV)

		go func() {
			<-quit

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := shutdown(ctx); err != nil {
				logger.Printf("Could not gracefully shutdown: %v\n", err)
			}
			close(done)
		}()

		<-done
		logger.Println("Server stopped")
	}
}
