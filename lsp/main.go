package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/owenrumney/go-lsp/server"
)

func main() {
	rpcPort := os.Getenv("JSON_RPC_PORT")
	wsPort := os.Getenv("WEBSOCKET_PORT")

	if rpcPort == "" && wsPort == "" {
		if err := runStdio(); err != nil {
			slog.Error("Stdio server failed", "error", err)
			os.Exit(1)
		}
		return
	}

	errChan := make(chan error, 1)

	if rpcPort != "" {
		slog.Info("Starting TCP server", "port", rpcPort)
		go func() {
			errChan <- runTCP(":" + rpcPort)
		}()
	}

	if wsPort != "" {

		slog.Info("Starting WebSocket server", "port", wsPort)
		go func() {
			errChan <- runWS(":" + wsPort)
		}()
	}

	// Wait for any server to fail
	err := <-errChan
	if err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

func runStdio() error {
	handler := &FMLHandler{}
	srv := server.NewServer(handler, server.WithLogger(slog.Default()))
	return srv.Run(context.Background(), server.RunStdio())
}

func runTCP(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	slog.Info("LSP server listening (TCP)", "addr", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Error("failed to accept connection", "error", err)
			continue
		}

		go func(c net.Conn) {
			defer c.Close()
			handler := &FMLHandler{}
			srv := server.NewServer(handler, server.WithLogger(slog.Default()))
			if err := srv.Run(context.Background(), c); err != nil {
				slog.Error("connection closed with error", "error", err)
			}
		}(conn)
	}
}

func runWS(addr string) error {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Error("failed to upgrade to websocket", "error", err)
			return
		}
		slog.Info("new websocket connection")

		go func() {
			defer conn.Close()
			handler := &FMLHandler{}
			srv := server.NewServer(handler, server.WithLogger(slog.Default()))
			if err := srv.Run(context.Background(), newWSConn(conn)); err != nil {
				slog.Error("websocket session closed with error", "error", err)
			}
		}()
	})

	slog.Info("LSP server listening (WebSocket)", "addr", addr)
	return http.ListenAndServe(addr, nil)
}
