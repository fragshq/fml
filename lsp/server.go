package lsp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/owenrumney/go-lsp/server"
)

func RunStdio(ctx context.Context) error {
	handler := &FMLHandler{}
	srv := server.NewServer(handler, server.WithLogger(slog.Default()))
	return srv.Run(ctx, server.RunStdio())
}

func RunTCP(ctx context.Context, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	slog.Info("LSP server listening (TCP)", "addr", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				slog.Error("failed to accept connection", "error", err)
				continue
			}
		}

		go func(c net.Conn) {
			defer func() {
				_ = c.Close()
			}()
			handler := &FMLHandler{}
			srv := server.NewServer(handler, server.WithLogger(slog.Default()))
			if err := srv.Run(ctx, c); err != nil {
				slog.Error("connection closed with error", "error", err)
			}
		}(conn)
	}
}

func RunWS(ctx context.Context, addr string) error {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Error("failed to upgrade to websocket", "error", err)
			return
		}
		slog.Info("new websocket connection")

		go func() {
			defer func() {
				_ = conn.Close()
			}()
			handler := &FMLHandler{}
			srv := server.NewServer(handler, server.WithLogger(slog.Default()))
			if err := srv.Run(ctx, newWSConn(conn)); err != nil {
				slog.Error("websocket session closed with error", "error", err)
			}
		}()
	})

	slog.Info("LSP server listening (WebSocket)", "addr", addr)

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	return srv.ListenAndServe()
}
