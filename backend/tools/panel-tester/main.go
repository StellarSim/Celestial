package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

type PanelMessage struct {
	PanelID string      `json:"panel_id"`
	Action  string      `json:"action"`
	Value   interface{} `json:"value"`
}

func main() {
	host := flag.String("host", "localhost", "Server host")
	port := flag.Int("port", 9090, "Server TCP port")
	flag.Parse()

	addr := fmt.Sprintf("%s:%d", *host, *port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	log.Printf("Connected to server at %s", addr)
	log.Println("Panel Testing Tool - Send commands to simulate ESP32 panel inputs")
	log.Println("Commands:")
	log.Println("  torpedo <bay_num> <action> [value]  - Control torpedo bay (actions: arm, load, lock, fire)")
	log.Println("  phaser <num> fire                   - Fire phaser")
	log.Println("  breaker <name> <on|off>             - Toggle power breaker")
	log.Println("  fire <location>                     - Extinguish fire")
	log.Println("  breach <location>                   - Seal breach")
	log.Println("  shields <on|off>                    - Toggle shields")
	log.Println("  alert <level>                       - Set alert level")
	log.Println("  quit                                - Exit tool")
	log.Println()

	go listenForResponses(conn)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if line == "quit" {
			break
		}

		msg := parseCommand(line)
		if msg == nil {
			fmt.Println("Invalid command")
			continue
		}

		data, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Error marshaling message: %v", err)
			continue
		}

		data = append(data, '\n')
		if _, err := conn.Write(data); err != nil {
			log.Printf("Error sending message: %v", err)
			break
		}

		log.Printf("Sent: %s", string(data))
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading input: %v", err)
	}
}

func parseCommand(line string) *PanelMessage {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}

	cmd := parts[0]

	switch cmd {
	case "torpedo":
		if len(parts) < 3 {
			return nil
		}
		bayNum := parts[1]
		action := parts[2]

		var value interface{}
		if action == "arm" || action == "lock" {
			if len(parts) >= 4 {
				value = parts[3] == "true" || parts[3] == "on"
			} else {
				value = true
			}
		}

		return &PanelMessage{
			PanelID: fmt.Sprintf("weapons_torpedo_%s", bayNum),
			Action:  fmt.Sprintf("%s_bay_%s", action, bayNum),
			Value:   value,
		}

	case "phaser":
		if len(parts) < 3 {
			return nil
		}
		num := parts[1]
		return &PanelMessage{
			PanelID: "weapons_phasers",
			Action:  fmt.Sprintf("fire_phaser_%s", num),
			Value:   nil,
		}

	case "breaker":
		if len(parts) < 3 {
			return nil
		}
		name := parts[1]
		enabled := parts[2] == "on" || parts[2] == "true"
		return &PanelMessage{
			PanelID: "engineer_power_1",
			Action:  fmt.Sprintf("breaker_%s", name),
			Value:   enabled,
		}

	case "fire":
		if len(parts) < 2 {
			return nil
		}
		location := parts[1]
		return &PanelMessage{
			PanelID: "engineer_damage_1",
			Action:  fmt.Sprintf("extinguish_%s", location),
			Value:   nil,
		}

	case "breach":
		if len(parts) < 2 {
			return nil
		}
		location := parts[1]
		return &PanelMessage{
			PanelID: "engineer_damage_1",
			Action:  fmt.Sprintf("seal_breach_%s", location),
			Value:   nil,
		}

	case "shields":
		if len(parts) < 2 {
			return nil
		}
		enabled := parts[1] == "on" || parts[1] == "true"
		return &PanelMessage{
			PanelID: "operations_power",
			Action:  "toggle_shields",
			Value:   enabled,
		}

	case "alert":
		if len(parts) < 2 {
			return nil
		}
		level := parts[1]
		return &PanelMessage{
			PanelID: "captain_alerts",
			Action:  fmt.Sprintf("%s_alert", level),
			Value:   level,
		}

	default:
		return nil
	}
}

func listenForResponses(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("Server response: %s", line)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Connection closed: %v", err)
	}
}
