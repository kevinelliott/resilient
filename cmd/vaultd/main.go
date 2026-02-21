package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"resilient/internal/api"
	"resilient/internal/daemon"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "inspect":
			inspectCmd := flag.NewFlagSet("inspect", flag.ExitOnError)
			inspectCmd.Parse(os.Args[2:])
			if inspectCmd.NArg() < 1 {
				fmt.Println("Usage: vaultd inspect <url>")
				os.Exit(1)
			}
			runInspectClient(inspectCmd.Arg(0))
			return
		case "import":
			importCmd := flag.NewFlagSet("import", flag.ExitOnError)
			strategy := importCmd.String("strategy", "hybrid", "Download strategy: http, mesh, hybrid")
			importCmd.Parse(os.Args[2:])
			if importCmd.NArg() < 1 {
				fmt.Println("Usage: vaultd import <url> [--strategy=hybrid|mesh|http]")
				os.Exit(1)
			}
			runImportClient(importCmd.Arg(0), *strategy)
			return
		}
	}

	// Default: Start Daemon
	dbPath := flag.String("db", "vault.db", "Path to the local SQLite database")
	casDir := flag.String("cas-dir", "vault_cas", "Path to the CAS storage directory")
	apiPort := flag.Int("api-port", 8080, "Port for the local HTTP API")
	p2pPort := flag.Int("p2p-port", 4001, "Port for libp2p networking")
	profile := flag.String("profile", "standard", "Node profile: standard, hub, or stealth")
	flag.Parse()

	log.Println("Starting vaultd...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the daemon
	d, err := daemon.New(ctx, &daemon.Config{
		DBPath:  *dbPath,
		CASDir:  *casDir,
		APIPort: *apiPort,
		P2PPort: *p2pPort,
		Profile: *profile,
	})
	if err != nil {
		log.Fatalf("Failed to initialize daemon: %v", err)
	}

	// Start the daemon
	if err := d.Start(); err != nil {
		log.Fatalf("Failed to start daemon: %v", err)
	}

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down vaultd...")
	if err := d.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
	log.Println("Shutdown complete.")
}

func runInspectClient(url string) {
	fmt.Printf("Inspecting RVX: %s\n", url)
	payload, _ := json.Marshal(map[string]string{"url": url})
	resp, err := http.Post("http://127.0.0.1:8080/api/inspect", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Fatalf("Failed to connect to local vaultd daemon: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Fatalf("Inspection failed: %s", string(body))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	
	header := result["header"].(map[string]interface{})
	mesh := result["mesh_availability"].(map[string]interface{})

	fmt.Printf("\n--- RVX METADATA ---\n")
	fmt.Printf("Type:  .%s\n", header["type"])
	
	metaData := header["metadata"].(map[string]interface{})
	if title, ok := metaData["title"]; ok {
		fmt.Printf("Title: %v\n", title)
	} else if name, ok := metaData["name"]; ok {
		fmt.Printf("Name:  %v\n", name)
	}
	
	fmt.Printf("\n--- MESH AVAILABILITY ---\n")
	fmt.Printf("Total Chunks Required: %v\n", mesh["total_chunks"])
	fmt.Printf("Local Chunks:          %v\n", mesh["local_chunks"])
	fmt.Printf("Mesh Peer Chunks:      %v\n", mesh["peer_chunks"])
	fmt.Printf("\nReady to import. Run 'vaultd import %s' to begin.\n", url)
}

func runImportClient(url string, strategy string) {
	fmt.Printf("Starting RVX Import (%s mode)...\n", strategy)
	
	// 1. Inspect first to get header
	payload, _ := json.Marshal(map[string]string{"url": url})
	resp, err := http.Post("http://127.0.0.1:8080/api/inspect", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Fatalf("Failed to connect to local vaultd daemon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Fatalf("Inspection failed: %s", string(body))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	
	// Convert arbitrary header map struct to strict JSON for API
	headerBytes, _ := json.Marshal(result["header"])
	var strictHeader api.RVXHeader
	json.Unmarshal(headerBytes, &strictHeader)

	// 2. Execute Import
	execPayload, _ := json.Marshal(map[string]interface{}{
		"url": url,
		"header": strictHeader,
		"strategy": strategy,
	})
	
	execResp, err := http.Post("http://127.0.0.1:8080/api/import/rvx", "application/json", bytes.NewBuffer(execPayload))
	if err != nil {
		log.Fatalf("Failed to execute import: %v", err)
	}
	defer execResp.Body.Close()

	if execResp.StatusCode == http.StatusAccepted {
		fmt.Println("Success: Native ingestion engine started securely in the background.")
	} else {
		body, _ := io.ReadAll(execResp.Body)
		fmt.Printf("Error: %s\n", string(body))
	}
}
