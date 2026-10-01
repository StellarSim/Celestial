package network

import (
	"celestial/internal/config"
	"celestial/internal/input"
	"celestial/internal/panel"
	"celestial/internal/ship"
	"celestial/internal/simulation"
	"encoding/json"
	"html"
	"net/http"
	"sort"
	"strings"
)

type WebPanelsHandler struct {
	mappings *config.PanelMapping
	sim      *simulation.Simulator
	router   *input.ActionRouter
	states   *panel.PanelStateManager
}

func NewWebPanelsHandler(mappings *config.PanelMapping, sim *simulation.Simulator, router *input.ActionRouter) *WebPanelsHandler {
	return &WebPanelsHandler{
		mappings: mappings,
		sim:      sim,
		router:   router,
		states:   panel.NewPanelStateManager(),
	}
}

// Mounts the index, per-panel pages and JSON API.
func (h *WebPanelsHandler) RegisterWebPanelRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/panels", h.handleIndex)
	mux.HandleFunc("/panels/", h.handlePanels)
	mux.HandleFunc("/api/panels", h.handleAPIList)
	mux.HandleFunc("/api/panels/", h.handleAPI)
}

func (h *WebPanelsHandler) panelIDs() []string {
	ids := make([]string, 0, len(h.mappings.Panels))
	for id := range h.mappings.Panels {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// splitPanelsPath parses /api/panels/{id}[/{rest}] and /panels/{id}.
func splitPanelsPath(p, prefix string) (id, rest string) {
	t := strings.TrimPrefix(p, prefix)
	t = strings.Trim(t, "/")
	if t == "" {
		return "", ""
	}
	parts := strings.SplitN(t, "/", 2)
	id = parts[0]
	if len(parts) == 2 {
		rest = parts[1]
	}
	return id, rest
}

func writeJSON(w http.ResponseWriter, status int, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	//nolint:errcheck // best effort for a status payload
	json.NewEncoder(w).Encode(obj)
}

func (h *WebPanelsHandler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/panels" && r.URL.Path != "/panels/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	//nolint:errcheck // best effort for a static page
	w.Write([]byte(h.renderIndex()))
}

func (h *WebPanelsHandler) handlePanels(w http.ResponseWriter, r *http.Request) {
	id, _ := splitPanelsPath(r.URL.Path, "/panels/")
	if id == "" {
		h.handleIndex(w, r)
		return
	}
	cfg, ok := h.mappings.Panels[id]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	//nolint:errcheck // best effort for a static page
	w.Write([]byte(h.renderPanel(id, cfg)))
}

func (h *WebPanelsHandler) handleAPIList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	type summary struct {
		ID      string   `json:"id"`
		Role    string   `json:"role"`
		Actions []string `json:"actions"`
	}
	out := make([]summary, 0, len(h.mappings.Panels))
	for _, id := range h.panelIDs() {
		cfg := h.mappings.Panels[id]
		actions := make([]string, 0, len(cfg.Actions))
		for name := range cfg.Actions {
			actions = append(actions, name)
		}
		sort.Strings(actions)
		out = append(out, summary{ID: id, Role: cfg.Role, Actions: actions})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"panels": out})
}

func (h *WebPanelsHandler) handleAPI(w http.ResponseWriter, r *http.Request) {
	id, rest := splitPanelsPath(r.URL.Path, "/api/panels/")
	cfg, ok := h.mappings.Panels[id]
	if !ok || id == "" {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"type": "error", "message": "unknown panel_id",
		})
		return
	}
	switch {
	case rest == "" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, cfg)
	case rest == "state" && r.Method == http.MethodGet:
		h.serveState(w, id)
	case rest == "action" && r.Method == http.MethodPost:
		h.serveAction(w, r, id, cfg)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (h *WebPanelsHandler) playerSnapshot() *ship.Ship {
	for _, sh := range h.sim.GetAllShips() {
		if sh.IsPlayer {
			return sh.Clone()
		}
	}
	return nil
}

func (h *WebPanelsHandler) stationState() panel.StationState {
	scanActive, scanTarget, scanProgress, scanMode := h.router.ScanState()
	hailing, hailTarget, frequency := h.router.CommState()
	transporterActive, transporterEmergency := h.router.TransporterState()
	return panel.StationState{
		ScanActive: scanActive, ScanTarget: scanTarget,
		ScanProgress: scanProgress, ScanMode: scanMode,
		Hailing: hailing, HailTarget: hailTarget, Frequency: frequency,
		TransporterActive: transporterActive, TransporterEmergency: transporterEmergency,
	}
}

func (h *WebPanelsHandler) serveState(w http.ResponseWriter, panelID string) {
	snapshot := h.playerSnapshot()
	if snapshot == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"type": "error", "message": "no player ship",
		})
		return
	}
	state := h.states.UpdateFromShip(panelID, snapshot, h.sim.GetCurrentTime(), h.stationState())
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"type": "state_update", "state": state,
	})
}

func (h *WebPanelsHandler) serveAction(w http.ResponseWriter, r *http.Request, panelID string, cfg config.PanelConfig) {
	var body struct {
		Action string      `json:"action"`
		Value  interface{} `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"type": "feedback", "panel_id": panelID, "status": "error", "message": "invalid JSON",
		})
		return
	}
	if body.Action == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"type": "feedback", "panel_id": panelID, "status": "error", "message": "action is required",
		})
		return
	}
	def, ok := cfg.Actions[body.Action]
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"type": "feedback", "panel_id": panelID, "status": "error",
			"message": "unknown action for panel",
		})
		return
	}
	action := &input.Action{
		Role:   cfg.Role,
		System: def.System,
		Action: def.Action,
		Value:  mergeValue(def.Value, body.Value),
	}
	if err := h.router.RouteAction(action); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"type": "feedback", "panel_id": panelID, "status": "error", "message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"type": "feedback", "panel_id": panelID, "status": "success", "action": body.Action,
	})
}

func (h *WebPanelsHandler) renderIndex() string {
	var b strings.Builder
	b.WriteString("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\">")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	b.WriteString("<title>Celestial Panels</title>")
	b.WriteString(indexCSS)
	b.WriteString("</head><body><main>")
	b.WriteString("<h1>Celestial Panels</h1>")
	b.WriteString("<p>One page per physical panel. Controls send through the same action catalog as the station screens and ESP32 panels.</p>")
	b.WriteString("<ul class=\"panel-list\">")
	for _, id := range h.panelIDs() {
		cfg := h.mappings.Panels[id]
		b.WriteString("<li><a href=\"/panels/" + html.EscapeString(id) + "\">")
		b.WriteString(html.EscapeString(id))
		b.WriteString("<span>" + html.EscapeString(cfg.Role) + "</span></a></li>")
	}
	b.WriteString("</ul></main></body></html>")
	return b.String()
}

func (h *WebPanelsHandler) renderPanel(id string, cfg config.PanelConfig) string {
	names := make([]string, 0, len(cfg.Actions))
	for name := range cfg.Actions {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\">")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	b.WriteString("<title>" + html.EscapeString(id) + " - Celestial Panel</title>")
	b.WriteString(panelCSS)
	b.WriteString("</head><body><main>")
	b.WriteString("<header><a href=\"/panels\">&lt; Panels</a>")
	b.WriteString("<h1>" + html.EscapeString(id) + "</h1>")
	b.WriteString("<p class=\"role\">role: " + html.EscapeString(cfg.Role) + "</p></header>")

	b.WriteString("<section class=\"targets\">")
	b.WriteString("<label>Target ID <input id=\"target\" type=\"text\" placeholder=\"optional target_id\" autocomplete=\"off\"></label>")
	b.WriteString("<label>Value JSON <input id=\"value\" type=\"text\" placeholder='optional, e.g. {\"frequency\": 121.5}' autocomplete=\"off\"></label>")
	b.WriteString("<p class=\"hint\">Some inputs need a target (hail, scan, torpedo fire). Type it once here; it is merged with the panel mapping when sent.</p>")
	b.WriteString("</section>")

	b.WriteString("<section><h2>Controls</h2><div class=\"controls\">")
	for _, name := range names {
		def := cfg.Actions[name]
		label := name + " (" + def.System + "." + def.Action + ")"
		b.WriteString("<button type=\"button\" data-action=\"" + html.EscapeString(name) + "\">")
		b.WriteString(html.EscapeString(label))
		b.WriteString("</button>")
	}
	b.WriteString("</div><p id=\"feedback\" role=\"status\"></p></section>")

	b.WriteString("<section><h2>State</h2>")
	b.WriteString("<div class=\"state-cols\"><div><h3>Indicators</h3><dl id=\"indicators\"></dl></div>")
	b.WriteString("<div><h3>Displays</h3><dl id=\"displays\"></dl></div></div></section>")
	b.WriteString("</main>")
	b.WriteString("<script>window.CELESTIAL_PANEL_ID=" + jsonString(id) + ";</script>")
	b.WriteString(panelJS)
	b.WriteString("</body></html>")
	return b.String()
}

func jsonString(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}

const indexCSS = `<style>
body{background:#060b16;color:#cfe3ff;font-family:system-ui,sans-serif;margin:0}
main{max-width:720px;margin:0 auto;padding:24px}
h1{color:#7fb4ff}
p{color:#9db4d4}
.panel-list{list-style:none;padding:0;display:grid;gap:12px}
.panel-list a{display:flex;justify-content:space-between;align-items:center;background:#0d1830;border:1px solid #24406e;border-radius:10px;color:#dff0ff;text-decoration:none;padding:18px;min-height:48px}
.panel-list span{color:#7fa3d8;font-size:14px}
</style>`

const panelCSS = `<style>
body{background:#060b16;color:#cfe3ff;font-family:system-ui,sans-serif;margin:0}
main{max-width:900px;margin:0 auto;padding:16px 16px 48px}
header a{color:#7fb4ff}
h1{color:#7fb4ff;margin:8px 0 0;overflow-wrap:anywhere}
.role,.hint{color:#9db4d4}
.targets{display:grid;gap:8px;background:#0d1830;border:1px solid #24406e;border-radius:10px;padding:12px;margin:16px 0}
.targets label{display:grid;gap:4px}
.targets input{background:#060b16;border:1px solid #24406e;border-radius:8px;color:#dff0ff;padding:12px;font-size:16px}
.controls{display:grid;grid-template-columns:repeat(auto-fill,minmax(220px,1fr));gap:12px}
.controls button{background:#12325e;border:1px solid #2f5b9e;border-radius:10px;color:#eaf3ff;cursor:pointer;min-height:56px;padding:14px;font-size:15px;overflow-wrap:anywhere}
.controls button:active{background:#1b4a8a}
#feedback{min-height:24px;color:#9db4d4}
.state-cols{display:grid;grid-template-columns:1fr 1fr;gap:16px}
@media (max-width:640px){.state-cols{grid-template-columns:1fr}}
dl{background:#0d1830;border:1px solid #24406e;border-radius:10px;padding:12px;margin:0;display:grid;gap:8px}
dt{color:#7fa3d8;font-size:13px;overflow-wrap:anywhere}
dd{margin:0;font-family:ui-monospace,monospace;overflow-wrap:anywhere}
.led{display:inline-block;width:12px;height:12px;border-radius:50%;margin-right:8px;vertical-align:baseline}
.led.green{background:#35d07f}.led.yellow{background:#e8c33a}.led.red{background:#e5484d}.led.blue{background:#4d9fff}
.led.off{background:#33415c}
.blink{animation:blink 1s steps(2) infinite}
@keyframes blink{50%{opacity:.25}}
</style>`

const panelJS = `<script>
(function(){
var panelID = window.CELESTIAL_PANEL_ID;
var feedback = document.getElementById("feedback");
var indList = document.getElementById("indicators");
var dispList = document.getElementById("displays");

function showFeedback(text, isError) {
  feedback.textContent = text;
  feedback.style.color = isError ? "#ff9d9d" : "#9db4d4";
}

function dynamicValue() {
  var target = document.getElementById("target").value.trim();
  var raw = document.getElementById("value").value.trim();
  var value;
  if (raw !== "") {
    try { value = JSON.parse(raw); }
    catch (e) { showFeedback("Value is not valid JSON", true); return undefined; }
  }
  if (target !== "") {
    if (value === null || typeof value !== "object" || Array.isArray(value)) value = {};
    if (value.target_id === undefined) value.target_id = target;
  }
  return value === undefined ? null : value;
}

document.querySelectorAll("button[data-action]").forEach(function(btn){
  btn.addEventListener("click", function(){
    var value = dynamicValue();
    if (value === undefined) return;
    fetch("/api/panels/" + encodeURIComponent(panelID) + "/action", {
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({action: btn.getAttribute("data-action"), value: value})
    }).then(function(res){ return res.json().then(function(body){ return {ok: res.ok, body: body}; }); })
    .then(function(result){
      if (result.ok) showFeedback("Sent " + btn.getAttribute("data-action"), false);
      else showFeedback("Error: " + (result.body.message || "request failed"), true);
    }).catch(function(){ showFeedback("Error: request failed", true); });
  });
});

function ledClass(color, on) {
  if (!on) return "led off";
  if (color === "green" || color === "yellow" || color === "red" || color === "blue") return "led " + color;
  return "led green";
}

function renderState(state) {
  indList.innerHTML = "";
  var inds = state.indicators || {};
  Object.keys(inds).sort().forEach(function(key){
    var ind = inds[key];
    var dt = document.createElement("dt");
    dt.textContent = key;
    var dd = document.createElement("dd");
    var dot = document.createElement("span");
    var on = !!ind.value;
    dot.className = ledClass(ind.color, on) + (ind.blink && on ? " blink" : "");
    dd.appendChild(dot);
    dd.appendChild(document.createTextNode(String(ind.value)));
    indList.appendChild(dt);
    indList.appendChild(dd);
  });
  dispList.innerHTML = "";
  var disps = state.displays || {};
  Object.keys(disps).sort().forEach(function(key){
    var disp = disps[key];
    var dt = document.createElement("dt");
    dt.textContent = key;
    var dd = document.createElement("dd");
    dd.textContent = String(disp.value) + (disp.unit ? " " + disp.unit : "");
    dispList.appendChild(dt);
    dispList.appendChild(dd);
  });
}

function poll() {
  fetch("/api/panels/" + encodeURIComponent(panelID) + "/state")
    .then(function(res){ return res.json(); })
    .then(function(body){ if (body.state) renderState(body.state); })
    .catch(function(){});
}
poll();
setInterval(poll, 500);
})();
</script>`
