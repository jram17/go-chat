package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jram17/go-chat/internal/server"
)

func main() {
	port := flag.String("port", "9000", "server port")
	certFile := flag.String("cert", "certs/server.crt", "TLS certificate file")
	keyFile := flag.String("key", "certs/server.key", "TLS private key file")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	hub := server.NewHub(logger)
	go hub.Run()

	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		logger.Error("failed to load TLS certificates", "err", err)
		os.Exit(1)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	listener, err := tls.Listen("tcp", ":"+*port, tlsConfig)
	if err != nil {
		logger.Error("failed to start listener", "err", err)
		os.Exit(1)
	}
	defer listener.Close()

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		logger.Info("shutting down server")
		listener.Close()
	}()

	logger.Info("server started", "port", *port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				logger.Error("accept error", "err", err)
				continue
			}
		}

		client := server.NewClient(conn, hub, logger)
		go client.WritePump()
		go client.ReadPump()

		logger.Info("new connection", "addr", conn.RemoteAddr().(*net.TCPAddr).String())
	}
}
