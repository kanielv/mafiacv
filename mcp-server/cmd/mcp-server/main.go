package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "modernc.org/sqlite"

	"github.com/kanielv/mafiacv/mcp-server/internal/rpc"
	"github.com/kanielv/mafiacv/mcp-server/internal/storage"
	"github.com/kanielv/mafiacv/mcp-server/internal/tools"
)

func main() {
	// CRITICAL: all logs must go to stderr. stdout is reserved for JSON-RPC
	// frames; anything else there corrupts the protocol.
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	dbPath := os.Getenv("MCP_DB_PATH")
	if dbPath == "" {
		dbPath = "mcp-server.db"
	}

	store, err := storage.New(dbPath)
	if err != nil {
		log.Fatalf("mcp-server: open storage: %v", err)
	}
	defer func() { _ = store.Close() }()

	registry := tools.New(store)
	for _, t := range registry.List() {
		log.Printf("mcp-server: registered tool %q", t.Name)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := rpc.NewServer(registry, os.Stdout)
	log.Println("mcp-server: stdio JSON-RPC transport ready")
	if err := server.Serve(ctx, os.Stdin); err != nil && ctx.Err() == nil {
		log.Fatalf("mcp-server: serve: %v", err)
	}
	log.Println("mcp-server: shutdown")
}
