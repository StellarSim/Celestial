extends Node
## WebSocket client for communicating with the Celestial backend server.
## Handles connection, reconnection, message sending and receiving.

signal connected
signal disconnected
signal connection_error(message: String)
signal message_received(data: Dictionary)
signal registered

enum ConnectionState {
	DISCONNECTED,
	CONNECTING,
	CONNECTED,
	REGISTERED
}

const HEARTBEAT_INTERVAL := 5.0
const RECONNECT_BASE_DELAY := 1.0
const RECONNECT_MAX_DELAY := 30.0

var state: ConnectionState = ConnectionState.DISCONNECTED
var server_url: String = "ws://localhost:8080"

var _ws: WebSocketPeer
var _reconnect_delay: float = RECONNECT_BASE_DELAY
var _heartbeat_timer: float = 0.0
var _reconnect_timer: float = 0.0
var _should_reconnect: bool = true

# Network statistics
var latency_ms: float = 0.0
var packets_received: int = 0
var packets_sent: int = 0
var last_receive_time: float = 0.0
var _ping_send_time: float = 0.0


func _ready() -> void:
	server_url = Config.get_server_url()


func _process(delta: float) -> void:
	if _ws == null:
		_handle_reconnect(delta)
		return
	
	_ws.poll()
	
	var ws_state := _ws.get_ready_state()
	
	match ws_state:
		WebSocketPeer.STATE_OPEN:
			if state == ConnectionState.CONNECTING:
				_on_connected()
			_process_messages()
			_process_heartbeat(delta)
		
		WebSocketPeer.STATE_CLOSING:
			pass
		
		WebSocketPeer.STATE_CLOSED:
			var code := _ws.get_close_code()
			var reason := _ws.get_close_reason()
			_on_disconnected(code, reason)
			_ws = null


func connect_to_server(url: String = "") -> void:
	if not url.is_empty():
		server_url = url
	
	if _ws != null:
		_ws.close()
		_ws = null
	
	state = ConnectionState.CONNECTING
	_ws = WebSocketPeer.new()
	
	var err := _ws.connect_to_url(server_url)
	if err != OK:
		state = ConnectionState.DISCONNECTED
		connection_error.emit("Failed to initiate connection: " + str(err))
		_ws = null


func disconnect_from_server() -> void:
	_should_reconnect = false
	if _ws != null:
		_ws.close()


func register(role: String, client_id: String = "") -> void:
	if client_id.is_empty():
		client_id = GameState.client_id
	
	var message := {
		"type": "register",
		"role": role,
		"client_id": client_id
	}
	send_message(message)
	
	GameState.client_role = role
	GameState.is_gm = role == "gm"
	GameState.is_display = role in ["tactical_display", "rear_display", "viewscreen"]


func send_message(data: Dictionary) -> bool:
	if _ws == null or state < ConnectionState.CONNECTED:
		return false
	
	var json := JSON.stringify(data)
	var err := _ws.send_text(json)
	
	if err == OK:
		packets_sent += 1
		return true
	
	return false


func send_action(system: String, action: String, value: Variant = 1) -> void:
	var message := {
		"type": "action",
		"role": GameState.client_role,
		"system": system,
		"action": action,
		"value": value
	}
	send_message(message)


func send_gm_command(command: String, params: Dictionary = {}) -> void:
	if not GameState.is_gm:
		push_warning("Attempted to send GM command without GM role")
		return
	
	var message := {
		"type": "gm_command",
		"command": command
	}
	message.merge(params)
	send_message(message)


func send_heartbeat() -> void:
	_ping_send_time = Time.get_ticks_msec()
	send_message({"type": "heartbeat"})


func is_connected_and_registered() -> bool:
	return state == ConnectionState.REGISTERED


func get_connection_status() -> String:
	match state:
		ConnectionState.DISCONNECTED:
			return "Disconnected"
		ConnectionState.CONNECTING:
			return "Connecting..."
		ConnectionState.CONNECTED:
			return "Connected"
		ConnectionState.REGISTERED:
			return "Online"
	return "Unknown"


func _on_connected() -> void:
	state = ConnectionState.CONNECTED
	_reconnect_delay = RECONNECT_BASE_DELAY
	_heartbeat_timer = 0.0
	connected.emit()
	print("[Network] Connected to server: ", server_url)


func _on_disconnected(code: int, reason: String) -> void:
	var was_connected := state >= ConnectionState.CONNECTED
	state = ConnectionState.DISCONNECTED
	
	if was_connected:
		print("[Network] Disconnected: ", code, " - ", reason)
		disconnected.emit()
	
	if _should_reconnect:
		_reconnect_timer = _reconnect_delay


func _process_messages() -> void:
	while _ws.get_available_packet_count() > 0:
		var packet := _ws.get_packet()
		var text := packet.get_string_from_utf8()
		
		var json := JSON.new()
		var err := json.parse(text)
		
		if err != OK:
			push_warning("[Network] Failed to parse message: ", json.get_error_message())
			continue
		
		var data: Dictionary = json.data
		packets_received += 1
		last_receive_time = Time.get_ticks_msec() / 1000.0
		
		_handle_message(data)


func _handle_message(data: Dictionary) -> void:
	var msg_type: String = data.get("type", "")
	
	match msg_type:
		"state_update":
			GameState.apply_state_update(data)
		
		"heartbeat":
			if _ping_send_time > 0:
				latency_ms = Time.get_ticks_msec() - _ping_send_time
				_ping_send_time = 0.0
		
		"feedback":
			if data.get("status") == "registered":
				state = ConnectionState.REGISTERED
				registered.emit()
				print("[Network] Registered as: ", GameState.client_role)
		
		"mission_event":
			var event_name: String = data.get("event", "")
			var event_data: Dictionary = data.get("data", {})
			GameState.mission_event.emit(event_name, event_data)
		
		"error":
			var error_msg: String = data.get("message", "Unknown error")
			push_warning("[Network] Server error: ", error_msg)
	
	message_received.emit(data)


func _process_heartbeat(delta: float) -> void:
	_heartbeat_timer += delta
	if _heartbeat_timer >= HEARTBEAT_INTERVAL:
		_heartbeat_timer = 0.0
		send_heartbeat()


func _handle_reconnect(delta: float) -> void:
	if not _should_reconnect or state != ConnectionState.DISCONNECTED:
		return
	
	_reconnect_timer -= delta
	if _reconnect_timer <= 0:
		print("[Network] Attempting reconnection...")
		connect_to_server()
		_reconnect_delay = minf(_reconnect_delay * 2, RECONNECT_MAX_DELAY)
