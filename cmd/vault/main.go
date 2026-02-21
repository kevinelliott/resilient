package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"flag"

	"resilient/internal/config"
)

var apiUrl string

func main() {
	configPath := flag.String("config", "", "Path to a YAML configuration file")
	apiOverride := flag.String("api-url", "", "Override the Daemon API URL")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("Configuration error: %v\n", err)
		os.Exit(1)
	}

	apiUrl = cfg.Client.APIUrl
	if *apiOverride != "" {
		apiUrl = *apiOverride
	}

	args := flag.Args()
	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	command := args[0]

	switch command {
	case "status":
		handleStatus()
	case "catalogs":
		handleCatalogs()
	case "chat":
		handleChat(args[1:])
	case "peers":
		handlePeers()
	case "inspect":
		handleInspect(args[1:])
	case "import":
		handleImport(args[1:])
	case "bootstrap":
		handleBootstrap()
	case "config":
		handleConfig()
	case "tui":
		LaunchTUI()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Resilient Knowledge Vault CLI")
	fmt.Println("\nUsage:")
	fmt.Println("  vault [flags] <command> [arguments]")
	fmt.Println("\nFlags:")
	fmt.Println("  -config string    Path to configuration file")
	fmt.Println("  -api-url string   Override configuration API endpoint (default: http://127.0.0.1:8080)")
	fmt.Println("\nCommands:")
	fmt.Println("  status            Check the status of the local daemon")
	fmt.Println("  catalogs          List all available catalogs")
	fmt.Println("  peers             List connected mesh peers")
	fmt.Println("  config            View the current node configuration")
	fmt.Println("  bootstrap         Trigger a fresh DHT network bootstrap")
	fmt.Println("  inspect <url>     Inspect a remote RVX payload")
	fmt.Println("  import <url>      Import an RVX payload via the mesh")
	fmt.Println("  chat <msg>        Transmit a message to the mesh")
	fmt.Println("  tui               Launch the interactive TUI mode")
}

func handleStatus() {
	resp, err := http.Get(apiUrl + "/api/info")
	if err != nil {
		fmt.Printf("Error contacting daemon: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Daemon returned status: %s\n", resp.Status)
		return
	}

	body, _ := io.ReadAll(resp.Body)
	var status map[string]interface{}
	json.Unmarshal(body, &status)

	fmt.Printf("Vault Daemon Status: %s\n", status["status"])
	fmt.Printf("Node ID: %s\n", status["node_id"])
	fmt.Printf("Version: %s\n", status["version"])
}

func handleCatalogs() {
	resp, err := http.Get(apiUrl + "/api/catalogs")
	if err != nil {
		fmt.Printf("Error contacting daemon: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var catalogs []map[string]interface{}
	json.Unmarshal(body, &catalogs)

	fmt.Printf("Available Catalogs:\n")
	fmt.Printf("-------------------\n")
	for _, c := range catalogs {
		hash := c["root_hash"].(string)
		if len(hash) > 10 {
			hash = hash[:10]
		}
		fmt.Printf("- %s (ID: %s) [Root: %s...]\n", c["name"], c["id"], hash)
	}
}

func handleChat(args []string) {
	if len(args) == 0 {
		fmt.Println("Error: please provide a message to send.")
		return
	}

	msg := args[0]
	for i := 1; i < len(args); i++ {
		msg += " " + args[i]
	}

	payloadBytes, _ := json.Marshal(map[string]string{"message": msg})
	resp, err := http.Post(apiUrl+"/api/chat", "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		fmt.Printf("Error contacting daemon: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Println("Message transmitted to the mesh.")
	} else {
		fmt.Printf("Failed to transmit. Status: %s\n", resp.Status)
	}
}

func handlePeers() {
	resp, err := http.Get(apiUrl + "/api/peers")
	if err != nil {
		fmt.Printf("Error contacting daemon: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var peers []map[string]interface{}
	json.Unmarshal(body, &peers)

	fmt.Printf("Connected Mesh Peers:\n")
	fmt.Printf("---------------------\n")
	for _, p := range peers {
		name := ""
		if n, ok := p["name"].(string); ok && n != "" {
			name = n + " "
		}
		fmt.Printf("- %s(ID: %s) [%s]\n", name, p["id"], p["status"])
	}
}

func handleConfig() {
	resp, err := http.Get(apiUrl + "/api/config")
	if err != nil {
		fmt.Printf("Error contacting daemon: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Daemon Configuration:\n%s\n", string(body))
}

func handleBootstrap() {
	resp, err := http.Post(apiUrl+"/api/bootstrap", "application/json", nil)
	if err != nil {
		fmt.Printf("Error contacting daemon: %v\n", err)
		return
	}
	defer resp.Body.Close()
	fmt.Println("Triggered DHT Bootstrap cycle.")
}

func handleInspect(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: vault inspect <url>")
		return
	}
	payload, _ := json.Marshal(map[string]string{"url": args[0]})
	resp, err := http.Post(apiUrl+"/api/inspect", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Printf("Error contacting daemon: %v\n", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("%s\n", string(body))
}

func handleImport(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: vault import <url>")
		return
	}
	payload, _ := json.Marshal(map[string]string{"url": args[0]})
	resp, err := http.Post(apiUrl+"/api/import/rvx", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Printf("Error contacting daemon: %v\n", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("%s\n", string(body))
}
