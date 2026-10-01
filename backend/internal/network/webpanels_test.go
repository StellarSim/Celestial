package network

import (
	"celestial/internal/input"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestWebPanels(t *testing.T) *WebPanelsHandler {
	t.Helper()
	mappings := loadPanelMappings(t)
	sim := newTestSim()
	router := input.NewActionRouter(sim)
	return NewWebPanelsHandler(mappings, sim, router)
}

func serveWebPanels(h *WebPanelsHandler) *httptest.Server {
	mux := http.NewServeMux()
	h.RegisterWebPanelRoutes(mux)
	return httptest.NewServer(mux)
}

func TestWebPanelsListCoversCanonicalIDs(t *testing.T) {
	h := newTestWebPanels(t)
	server := serveWebPanels(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/panels")
	if err != nil {
		t.Fatalf("GET /api/panels: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/panels status = %d", resp.StatusCode)
	}
	var body struct {
		Panels []struct {
			ID      string   `json:"id"`
			Role    string   `json:"role"`
			Actions []string `json:"actions"`
		} `json:"panels"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Panels) != len(canonicalPanelIDs) {
		t.Fatalf("expected %d panels, got %d", len(canonicalPanelIDs), len(body.Panels))
	}
	seen := map[string]bool{}
	for _, p := range body.Panels {
		seen[p.ID] = true
		if p.Role == "" {
			t.Errorf("panel %s has no role", p.ID)
		}
		if len(p.Actions) == 0 {
			t.Errorf("panel %s has no actions", p.ID)
		}
	}
	for _, id := range canonicalPanelIDs {
		if !seen[id] {
			t.Errorf("missing canonical panel %q", id)
		}
	}
}

func TestWebPanelPagePerID(t *testing.T) {
	h := newTestWebPanels(t)
	server := serveWebPanels(h)
	defer server.Close()

	for _, id := range canonicalPanelIDs {
		resp, err := http.Get(server.URL + "/panels/" + id)
		if err != nil {
			t.Fatalf("GET /panels/%s: %v", id, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET /panels/%s status = %d", id, resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			t.Errorf("GET /panels/%s content-type = %q", id, ct)
		}
		resp.Body.Close()
	}

	resp, err := http.Get(server.URL + "/panels")
	if err != nil {
		t.Fatalf("GET /panels: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /panels status = %d", resp.StatusCode)
	}

	resp, err = http.Get(server.URL + "/panels/not_a_panel")
	if err != nil {
		t.Fatalf("GET unknown panel: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown panel status = %d, want 404", resp.StatusCode)
	}
}

func TestWebPanelStateShape(t *testing.T) {
	h := newTestWebPanels(t)
	server := serveWebPanels(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/panels/engineer_power_main/state")
	if err != nil {
		t.Fatalf("GET state: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET state status = %d", resp.StatusCode)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["type"] != "state_update" {
		t.Errorf("type = %v, want state_update", body["type"])
	}
	state, ok := body["state"].(map[string]interface{})
	if !ok {
		t.Fatal("state must be an object")
	}
	if state["panel_id"] != "engineer_power_main" {
		t.Errorf("panel_id = %v", state["panel_id"])
	}
	if _, ok := state["indicators"]; !ok {
		t.Error("state must carry indicators")
	}
	if _, ok := state["displays"]; !ok {
		t.Error("state must carry displays")
	}
}

func TestWebPanelActionRoundTrip(t *testing.T) {
	h := newTestWebPanels(t)
	server := serveWebPanels(h)
	defer server.Close()

	post := func(panelID, action string, value interface{}) (int, map[string]interface{}) {
		t.Helper()
		payload := map[string]interface{}{"action": action}
		if value != nil {
			payload["value"] = value
		}
		data, _ := json.Marshal(payload)
		resp, err := http.Post(server.URL+"/api/panels/"+panelID+"/action", "application/json", strings.NewReader(string(data)))
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer resp.Body.Close()
		var body map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		return resp.StatusCode, body
	}

	status, body := post("captain_command", "alert_red", nil)
	if status != http.StatusOK {
		t.Fatalf("alert_red status = %d (%v)", status, body)
	}
	if body["status"] != "success" {
		t.Errorf("alert_red feedback = %v", body)
	}
	if h.sim.GetAlertLevel() != "red" {
		t.Errorf("alert level = %q, want red", h.sim.GetAlertLevel())
	}

	status, _ = post("captain_command", "no_such_button", nil)
	if status != http.StatusBadRequest {
		t.Errorf("unknown action status = %d, want 400", status)
	}

	status, _ = post("not_a_panel", "alert_red", nil)
	if status != http.StatusNotFound {
		t.Errorf("unknown panel status = %d, want 404", status)
	}

	status, _ = post("captain_command", "alert_red", "not-json{{{")
	if status != http.StatusBadRequest && status != http.StatusOK {
		t.Errorf("unexpected status for raw value: %d", status)
	}
}
