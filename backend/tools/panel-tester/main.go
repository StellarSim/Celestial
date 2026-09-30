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
	"time"
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

	addr := net.JoinHostPort(*host, fmt.Sprint(*port))
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	log.Printf("Connected to server at %s", addr)
	log.Println("Panel Testing Tool - send panel-local inputs over the TCP protocol")
	log.Println("Commands (panel_id and action names match backend/configs/panels.yaml):")
	log.Println("  register <panel_id>")
	log.Println("  breaker <name>                 - toggle a power breaker (engineer_power_main)")
	log.Println("  repair <section>               - repair forward|aft|port|starboard (engineer_damage_main)")
	log.Println("  extinguish <section>           - extinguish fire in a section")
	log.Println("  seal <section>                 - seal a breach in a section")
	log.Println("  deploy <team 0-2>              - deploy a repair team")
	log.Println("  recall <team 0-2>              - recall a repair team")
	log.Println("  throttle <value>               - set throttle, -1.0..1.0 (flight_main)")
	log.Println("  turn <rate>                    - set turn rate (flight_main)")
	log.Println("  torpedo <bay 1-4> <arm|load|lock|unlock|fire> [target_id]")
	log.Println("  phaser <1|2> [target_id]       - fire a phaser array (weapons_phasers)")
	log.Println("  auto_fire <on|off>             - toggle auto fire")
	log.Println("  shields <up|down>              - raise/lower shields (operations_power)")
	log.Println("  transporter <up|down|emergency>")
	log.Println("  sensors <passive|active|deep>  - set sensor mode (relay_sensors)")
	log.Println("  scan <target_id>               - initiate scan (relay_scanning)")
	log.Println("  deep_scan <target_id>          - deep scan a target")
	log.Println("  probe                          - launch a probe")
	log.Println("  hail <target_id>               - hail a ship (comms_main)")
	log.Println("  freq <frequency>               - set comms frequency (comms_main)")
	log.Println("  alert <normal|yellow|red>      - set alert level (captain_command)")
	log.Println("  self_destruct <arm|abort>")
	log.Println("  send <panel_id> <action> [value]  - send a raw panel input")
	log.Println("  quit")
	log.Println()

	responses := make(chan string, 64)
	go listenForResponses(conn, responses)

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

		msg, err := parseCommand(line)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}

		data, err := json.Marshal(msg)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}

		if _, err := conn.Write(append(data, '\n')); err != nil {
			fmt.Printf("connection error: %v\n", err)
			break
		}
	}

	// Let the server's replies arrive before the connection closes.
	fmt.Println()
	for {
		select {
		case line, ok := <-responses:
			if !ok {
				return
			}
			fmt.Printf("server: %s\n", line)
		case <-time.After(500 * time.Millisecond):
			if err := scanner.Err(); err != nil {
				log.Printf("Error reading input: %v", err)
			}
			return
		}
	}
}

func parseCommand(line string) (*PanelMessage, error) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmd := parts[0]
	arg := func(i int) string {
		if i < len(parts) {
			return parts[i]
		}
		return ""
	}
	requireArg := func(name string, i int) (string, error) {
		if i >= len(parts) {
			return "", fmt.Errorf("%s requires %s", cmd, name)
		}
		return parts[i], nil
	}

	switch cmd {
	case "register":
		id, err := requireArg("panel_id", 1)
		if err != nil {
			return nil, err
		}
		return &PanelMessage{PanelID: id, Action: "register"}, nil

	case "breaker":
		name, err := requireArg("breaker name", 1)
		if err != nil {
			return nil, err
		}
		if !contains(breakerNames, name) {
			return nil, fmt.Errorf("unknown breaker %q (valid: %s)", name, strings.Join(breakerNames, ", "))
		}
		return &PanelMessage{PanelID: "engineer_power_main", Action: "breaker_" + name}, nil

	case "repair":
		section, err := requireArg("section", 1)
		if err != nil {
			return nil, err
		}
		return &PanelMessage{PanelID: "engineer_damage_main", Action: "repair_" + section}, nil

	case "extinguish":
		section, err := requireArg("section", 1)
		if err != nil {
			return nil, err
		}
		return &PanelMessage{PanelID: "engineer_damage_main", Action: "extinguish_" + section}, nil

	case "seal":
		section, err := requireArg("section", 1)
		if err != nil {
			return nil, err
		}
		return &PanelMessage{PanelID: "engineer_damage_main", Action: "seal_" + section}, nil

	case "deploy", "recall":
		team, err := requireArg("team index", 1)
		if err != nil {
			return nil, err
		}
		idx, err := parseTeamIndex(team)
		if err != nil {
			return nil, err
		}
		name := "alpha"
		if idx == 1 {
			name = "beta"
		} else if idx == 2 {
			name = "gamma"
		}
		return &PanelMessage{PanelID: "engineer_systems", Action: cmd + "_" + name}, nil

	case "throttle":
		value, err := requireArg("throttle value", 1)
		if err != nil {
			return nil, err
		}
		return &PanelMessage{
			PanelID: "flight_main",
			Action:  throttleActionFor(value),
			Value:   value,
		}, nil

	case "turn":
		value, err := requireArg("turn rate", 1)
		if err != nil {
			return nil, err
		}
		action := "turn_steady"
		switch {
		case strings.HasPrefix(value, "-"):
			action = "turn_port"
		case value != "0" && value != "0.0":
			action = "turn_starboard"
		}
		return &PanelMessage{PanelID: "flight_main", Action: action, Value: value}, nil

	case "torpedo":
		bay, err := requireArg("bay number", 1)
		if err != nil {
			return nil, err
		}
		op, err := requireArg("operation", 2)
		if err != nil {
			return nil, err
		}
		panel := "weapons_torpedos_1"
		if bay == "3" || bay == "4" {
			panel = "weapons_torpedos_2"
		}
		var action string
		switch op {
		case "arm":
			action = "arm_bay_" + bay
		case "disarm":
			action = "disarm_bay_" + bay
		case "load":
			action = "load_bay_" + bay
		case "lock":
			action = "lock_bay_" + bay
		case "unlock":
			action = "unlock_bay_" + bay
		case "fire":
			action = "fire_bay_" + bay
		default:
			return nil, fmt.Errorf("unknown torpedo operation %q", op)
		}
		msg := &PanelMessage{PanelID: panel, Action: action}
		if target := arg(3); target != "" {
			msg.Value = map[string]interface{}{"target_id": target}
		}
		return msg, nil

	case "phaser":
		num, err := requireArg("array number", 1)
		if err != nil {
			return nil, err
		}
		msg := &PanelMessage{PanelID: "weapons_phasers", Action: "fire_array_" + num}
		if target := arg(2); target != "" {
			msg.Value = map[string]interface{}{"target_id": target}
		}
		return msg, nil

	case "auto_fire":
		state, err := requireArg("on or off", 1)
		if err != nil {
			return nil, err
		}
		action := "auto_fire_on"
		if state == "off" || state == "false" {
			action = "auto_fire_off"
		}
		return &PanelMessage{PanelID: "weapons_phasers", Action: action}, nil

	case "shields":
		dir, err := requireArg("up or down", 1)
		if err != nil {
			return nil, err
		}
		action := "shields_up"
		if dir == "down" {
			action = "shields_down"
		}
		return &PanelMessage{PanelID: "operations_power", Action: action}, nil

	case "transporter":
		dir, err := requireArg("up, down or emergency", 1)
		if err != nil {
			return nil, err
		}
		switch dir {
		case "up":
			return &PanelMessage{PanelID: "operations_resources", Action: "transporter_beam_up"}, nil
		case "down":
			return &PanelMessage{PanelID: "operations_resources", Action: "transporter_beam_down"}, nil
		case "emergency":
			return &PanelMessage{PanelID: "operations_resources", Action: "transporter_emergency"}, nil
		default:
			return nil, fmt.Errorf("unknown transporter mode %q", dir)
		}

	case "sensors":
		mode, err := requireArg("passive, active or deep", 1)
		if err != nil {
			return nil, err
		}
		switch mode {
		case "passive", "active", "deep":
			return &PanelMessage{PanelID: "relay_sensors", Action: "sensor_" + mode}, nil
		default:
			return nil, fmt.Errorf("unknown sensor mode %q", mode)
		}

	case "scan", "hail":
		target, err := requireArg("target_id", 1)
		if err != nil {
			return nil, err
		}
		panel := "relay_scanning"
		action := "scan_selected"
		if cmd == "hail" {
			panel = "comms_main"
			action = "hail_selected"
		}
		return &PanelMessage{PanelID: panel, Action: action, Value: map[string]interface{}{"target_id": target}}, nil

	case "deep_scan":
		target, err := requireArg("target_id", 1)
		if err != nil {
			return nil, err
		}
		return &PanelMessage{
			PanelID: "relay_scanning",
			Action:  "deep_scan_selected",
			Value:   map[string]interface{}{"target_id": target},
		}, nil

	case "probe":
		return &PanelMessage{PanelID: "relay_sensors", Action: "launch_probe"}, nil

	case "freq":
		value, err := requireArg("frequency", 1)
		if err != nil {
			return nil, err
		}
		return &PanelMessage{
			PanelID: "comms_main",
			Action:  freqActionFor(value),
			Value:   map[string]interface{}{"frequency": parseFloat(value)},
		}, nil

	case "alert":
		level, err := requireArg("level", 1)
		if err != nil {
			return nil, err
		}
		return &PanelMessage{PanelID: "captain_command", Action: "alert_" + level}, nil

	case "self_destruct":
		phase, err := requireArg("arm or abort", 1)
		if err != nil {
			return nil, err
		}
		switch phase {
		case "arm":
			return &PanelMessage{PanelID: "captain_command", Action: "self_destruct_arm"}, nil
		case "abort":
			return &PanelMessage{PanelID: "captain_command", Action: "self_destruct_abort"}, nil
		default:
			return nil, fmt.Errorf("unknown self destruct phase %q", phase)
		}

	case "send":
		id, err := requireArg("panel_id", 1)
		if err != nil {
			return nil, err
		}
		action, err := requireArg("action", 2)
		if err != nil {
			return nil, err
		}
		msg := &PanelMessage{PanelID: id, Action: action}
		if raw := arg(3); raw != "" {
			var v interface{}
			if err := json.Unmarshal([]byte(raw), &v); err != nil {
				v = raw
			}
			msg.Value = v
		}
		return msg, nil

	default:
		return nil, fmt.Errorf("unknown command %q (type 'help' input above for the list)", cmd)
	}
}

var breakerNames = []string{
	"reactor", "engines", "shields", "weapons",
	"sensors", "comms", "life_support", "navigation",
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func parseTeamIndex(s string) (int, error) {
	var idx int
	if _, err := fmt.Sscanf(s, "%d", &idx); err != nil {
		return 0, fmt.Errorf("team must be 0, 1 or 2")
	}
	if idx < 0 || idx > 2 {
		return 0, fmt.Errorf("team must be 0, 1 or 2")
	}
	return idx, nil
}

func parseFloat(s string) float64 {
	var f float64
	if _, err := fmt.Sscanf(s, "%g", &f); err != nil {
		return 0
	}
	return f
}

func throttleActionFor(value string) string {
	f := parseFloat(value)
	switch {
	case f >= 1.0:
		return "throttle_full"
	case f >= 0.5:
		return "throttle_half"
	case f > 0.0:
		return "throttle_up"
	case f == 0.0:
		return "throttle_all_stop"
	case f <= -1.0:
		return "throttle_full_reverse"
	case f <= -0.5:
		return "throttle_half_reverse"
	default:
		return "throttle_half_reverse"
	}
}

func freqActionFor(value string) string {
	f := parseFloat(value)
	switch {
	case f == 121.5:
		return "freq_emergency"
	case f == 243.0:
		return "freq_military"
	case f == 156.8:
		return "freq_civilian"
	default:
		return "freq_emergency"
	}
}

func listenForResponses(conn net.Conn, out chan<- string) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		out <- scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		out <- "connection closed: " + err.Error()
	}
	close(out)
}
