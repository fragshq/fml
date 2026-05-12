package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/owenrumney/go-lsp/server"
)

func main() {
	handler := &FMLHandler{}
	srv := server.NewServer(handler, server.WithLogger(slog.Default()))

	if err := srv.Run(context.Background(), server.RunStdio()); err != nil {
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}
