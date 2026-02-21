package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const defaultAPIUrl = "http://127.0.0.1:8080"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "status":
		handleStatus()
	case "catalogs":
		handleCatalogs()
	case "chat":
		handleChat(os.Args[2:])
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
	fmt.Println("  vault <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  status      Check the status of the local daemon")
	fmt.Println("  catalogs    List all available catalogs")
	fmt.Println("  chat <msg>  Transmit a message to the mesh")
	fmt.Println("  tui         Launch the interactive TUI mode")
}

func handleStatus() {
	resp, err := http.Get(defaultAPIUrl + "/api/info")
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
	resp, err := http.Get(defaultAPIUrl + "/api/catalogs")
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

	payload := map[string]string{
		"content":       msg,
		"ref_target_id": "",
	}
	data, _ := json.Marshal(payload)

	resp, err := http.Post(defaultAPIUrl+"/api/chat", "application/json", bytes.NewBuffer(data))
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
