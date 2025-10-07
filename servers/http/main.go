package main

import (
	"flag"
	"fmt"
	"os"

	sharedlog "github.com/dimasd-angga/go-mcp-servers/shared/logger"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	transport := flag.String("transport", "stdio", "Transport: stdio or sse")
	port := flag.Int("port", 8005, "Port for SSE transport")
	flag.Parse()

	log := sharedlog.Init("http")

	h, err := NewHTTPServer()
	if err != nil {
		log.Fatal().Err(err).Msg("init failed")
	}

	log.Info().
		Strs("allowed_hosts", h.AllowedHosts()).
		Int64("max_response", h.maxResponseSize).
		Str("transport", *transport).
		Msg("http MCP server starting")

	switch *transport {
	case "stdio":
		if err := server.ServeStdio(h.MCP()); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	case "sse":
		sse := server.NewSSEServer(h.MCP(),
			server.WithBaseURL(fmt.Sprintf("http://localhost:%d", *port)),
		)
		log.Info().Int("port", *port).Msg("listening on SSE")
		if err := sse.Start(fmt.Sprintf(":%d", *port)); err != nil {
			log.Fatal().Err(err).Msg("SSE server failed")
		}
	default:
		log.Fatal().Str("transport", *transport).Msg("unknown transport")
	}
}
