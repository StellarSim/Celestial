extends StationPanel
## Communications station panel - hailing, frequencies, and message handling.

const FILTER_OPTIONS := ["All", "Friendly", "Enemy", "Neutral", "Stations"]
const PRESET_FREQUENCIES := {
	"emergency": 121.5,
	"military": 243.0,
	"civilian": 156.8
}

@onready var filter_select: OptionButton = $MainSplit/LeftSection/ContactsSection/ContactsContent/ContactFilter/FilterSelect
@onready var contact_list: ItemList = $MainSplit/LeftSection/ContactsSection/ContactsContent/ContactList
@onready var hail_btn: Button = $MainSplit/LeftSection/ContactsSection/ContactsContent/ContactActions/HailContact
@onready var scan_btn: Button = $MainSplit/LeftSection/ContactsSection/ContactsContent/ContactActions/ScanContact

@onready var frequency_slider: HSlider = $MainSplit/LeftSection/FrequencySection/FrequencyContent/FrequencySlider
@onready var freq_value: Label = $MainSplit/LeftSection/FrequencySection/FrequencyContent/FrequencyDisplay/FreqValue
@onready var emergency_freq_btn: Button = $MainSplit/LeftSection/FrequencySection/FrequencyContent/PresetFrequencies/EmergencyFreq
@onready var military_freq_btn: Button = $MainSplit/LeftSection/FrequencySection/FrequencyContent/PresetFrequencies/MilitaryFreq
@onready var civilian_freq_btn: Button = $MainSplit/LeftSection/FrequencySection/FrequencyContent/PresetFrequencies/CivilianFreq

@onready var comm_log: RichTextLabel = $MainSplit/RightSection/CommLog/CommLogContent/CommLogText

@onready var identify_btn: Button = $MainSplit/RightSection/TransmitSection/TransmitContent/MessagePresets/IdentifyBtn
@onready var request_dock_btn: Button = $MainSplit/RightSection/TransmitSection/TransmitContent/MessagePresets/RequestDockBtn
@onready var mayday_btn: Button = $MainSplit/RightSection/TransmitSection/TransmitContent/MessagePresets/MaydayBtn
@onready var surrender_btn: Button = $MainSplit/RightSection/TransmitSection/TransmitContent/MessagePresets/SurrenderBtn

@onready var message_input: LineEdit = $MainSplit/RightSection/TransmitSection/TransmitContent/CustomMessage/MessageInput
@onready var send_btn: Button = $MainSplit/RightSection/TransmitSection/TransmitContent/CustomMessage/SendBtn

@onready var signal_bar: ProgressBar = $MainSplit/RightSection/TransmitStatus/StatusContent/SignalStrength/SignalBar
@onready var jamming_value: Label = $MainSplit/RightSection/TransmitStatus/StatusContent/JammingStatus/JammingValue

var _selected_contact_id: String = ""
var _current_frequency: float = 500.0
var _current_filter: String = "All"


func _ready() -> void:
	super._ready()
	_setup_filters()
	_connect_signals()


func _process(_delta: float) -> void:
	_update_display()


func _setup_filters() -> void:
	filter_select.clear()
	for opt in FILTER_OPTIONS:
		filter_select.add_item(opt)


func _connect_signals() -> void:
	GameState.mission_event.connect(_on_mission_event)
	
	filter_select.item_selected.connect(_on_filter_changed)
	contact_list.item_selected.connect(_on_contact_selected)
	
	hail_btn.pressed.connect(_on_hail)
	scan_btn.pressed.connect(_on_scan)
	
	frequency_slider.value_changed.connect(_on_frequency_changed)
	emergency_freq_btn.pressed.connect(func(): _set_frequency(PRESET_FREQUENCIES.emergency))
	military_freq_btn.pressed.connect(func(): _set_frequency(PRESET_FREQUENCIES.military))
	civilian_freq_btn.pressed.connect(func(): _set_frequency(PRESET_FREQUENCIES.civilian))
	
	identify_btn.pressed.connect(_on_identify)
	request_dock_btn.pressed.connect(_on_request_dock)
	mayday_btn.pressed.connect(_on_mayday)
	surrender_btn.pressed.connect(_on_surrender)
	
	send_btn.pressed.connect(_on_send_custom)
	message_input.text_submitted.connect(func(_t): _on_send_custom())


func _update_display() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	
	_update_contact_list()
	_update_frequency_display(ship)
	_update_signal_status(ship)


func _update_contact_list() -> void:
	var player_ship := GameState.get_player_ship()
	if player_ship == null:
		return

	contact_list.clear()

	for entry in NavUtils.contacts_by_distance():
		var ship: GameState.ShipState = entry.ship
		var ship_id: String = entry.id

		# Apply filter
		if not _passes_filter(ship):
			continue

		var faction_color: Color = GameState.get_faction_color(ship.faction)
		var display_text := "%s [%.1f km]" % [ship.name, entry.distance / 1000.0]

		var idx := contact_list.add_item(display_text)
		contact_list.set_item_custom_fg_color(idx, faction_color)
		contact_list.set_item_metadata(idx, ship_id)

		if ship_id == _selected_contact_id:
			contact_list.select(idx)

	hail_btn.disabled = _selected_contact_id.is_empty()
	scan_btn.disabled = _selected_contact_id.is_empty()


func _passes_filter(ship: GameState.ShipState) -> bool:
	var disposition := GameState.faction_disposition(ship.faction)
	match _current_filter:
		"All":
			return true
		"Friendly":
			return disposition == "friendly"
		"Enemy":
			return disposition == "hostile"
		"Neutral":
			return disposition == "neutral"
		"Stations":
			return ship.ship_class.to_lower().contains("station") or ship.ship_class.to_lower().contains("base")
	return true


func _update_frequency_display(ship: GameState.ShipState) -> void:
	# The tuned frequency is server state.
	_current_frequency = ship.communications.frequency
	frequency_slider.set_value_no_signal(_current_frequency)
	freq_value.text = "%.1f MHz" % _current_frequency


func _update_signal_status(ship: GameState.ShipState) -> void:
	var comms_online: bool = ship.communications.enabled and ship.power_breakers.get("comms", true)
	var signal_strength := 100.0 if comms_online else 0.0
	
	signal_bar.value = signal_strength
	signal_bar.modulate = Colors.get_health_color(signal_strength / 100.0)
	
	if ship.communications.hailing:
		jamming_value.text = "HAILING: %s" % ship.communications.hail_target
		jamming_value.add_theme_color_override("font_color", Colors.ALERT_YELLOW)
	else:
		jamming_value.text = "NO HAIL"
		jamming_value.add_theme_color_override("font_color", Colors.STATUS_ONLINE)


func _on_filter_changed(idx: int) -> void:
	_current_filter = FILTER_OPTIONS[idx]


func _on_contact_selected(idx: int) -> void:
	_selected_contact_id = contact_list.get_item_metadata(idx)
	hail_btn.disabled = false
	scan_btn.disabled = false


func _on_hail() -> void:
	if _selected_contact_id.is_empty():
		return
	
	var target: GameState.ShipState = GameState.ships.get(_selected_contact_id)
	if target == null:
		return
	
	NetworkClient.send_action("comms", "hail", {
		"target_id": _selected_contact_id,
		"frequency": _current_frequency
	})
	
	_add_log("OUTGOING", "Hailing %s on %.1f MHz..." % [target.name, _current_frequency])


func _on_scan() -> void:
	if _selected_contact_id.is_empty():
		return
	
	var target: GameState.ShipState = GameState.ships.get(_selected_contact_id)
	if target == null:
		return
	
	NetworkClient.send_action("comms", "deep_scan", {"target_id": _selected_contact_id})
	_add_log("SENSORS", "Initiating deep scan of %s..." % target.name)


func _on_frequency_changed(value: float) -> void:
	_current_frequency = value
	NetworkClient.send_action("comms", "set_frequency", {"frequency": value})


func _set_frequency(freq: float) -> void:
	_current_frequency = freq
	frequency_slider.set_value_no_signal(freq)
	NetworkClient.send_action("comms", "set_frequency", {"frequency": freq})


func _on_identify() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	
	NetworkClient.send_action("comms", "transmit", {
		"type": "identify",
		"frequency": _current_frequency
	})
	_add_log("OUTGOING", "This is %s, identifying on all frequencies." % ship.name)


func _on_request_dock() -> void:
	if _selected_contact_id.is_empty():
		_add_log("ERROR", "No target selected for docking request.")
		return
	
	NetworkClient.send_action("comms", "send_message", {
		"message": "Requesting docking clearance",
		"target_id": _selected_contact_id,
		"frequency": _current_frequency
	})
	_add_log("OUTGOING", "Requesting docking clearance...")


func _on_mayday() -> void:
	NetworkClient.send_action("comms", "broadcast", {
		"message": "MAYDAY MAYDAY MAYDAY",
		"frequency": PRESET_FREQUENCIES.emergency
	})
	_set_frequency(PRESET_FREQUENCIES.emergency)
	_add_log("EMERGENCY", "[color=red]MAYDAY MAYDAY MAYDAY - All stations please respond![/color]")


func _on_surrender() -> void:
	NetworkClient.send_action("comms", "broadcast", {
		"message": "We are surrendering. Ceasing all hostile actions.",
		"frequency": _current_frequency
	})
	_add_log("OUTGOING", "[color=yellow]We are surrendering. Ceasing all hostile actions.[/color]")


func _on_send_custom() -> void:
	var message := message_input.text.strip_edges()
	if message.is_empty():
		return
	
	NetworkClient.send_action("comms", "send_message", {
		"message": message,
		"frequency": _current_frequency,
		"target_id": _selected_contact_id
	})
	
	_add_log("OUTGOING", message)
	message_input.clear()


func _add_log(source: String, message: String) -> void:
	var time_str := Time.get_time_string_from_system()
	var color := Colors.PRIMARY
	
	match source:
		"INCOMING":
			color = Colors.ALERT_GREEN
		"OUTGOING":
			color = Colors.PRIMARY
		"ERROR":
			color = Colors.ALERT_RED
		"EMERGENCY":
			color = Colors.ALERT_RED
		"SENSORS":
			color = Colors.ALERT_YELLOW
	
	comm_log.append_text("[color=#%s][%s] [b]%s:[/b][/color] %s\n" % [
		color.to_html(false), time_str, source, message
	])



func _on_mission_event(event_name: String, data: Dictionary) -> void:
	var event_type: String = event_name
	if data.has("type"):
		event_type = str(data.get("type", event_name))

	if event_type == "incoming_comm":
		var source: String = str(data.get("source", "UNKNOWN"))
		var message: String = str(data.get("message", ""))
		_add_log("INCOMING", "[b]%s:[/b] %s" % [source, message])
	elif event_type == "mission_win":
		_add_log("MISSION", "[b]MISSION COMPLETE[/b]")
	elif event_type == "mission_lose":
		var reason: String = str(data.get("reason", data.get("message", "Defeat")))
		_add_log("MISSION", "[b]MISSION FAILED:[/b] %s" % reason)
