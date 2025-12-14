extends Control
## Captain's panel - ship overview, alerts, mission status, and command.

@onready var ship_name: Label = $MainSplit/LeftSection/ShipStatus/StatusContent/ShipName
@onready var hull_bar: ProgressBar = $MainSplit/LeftSection/ShipStatus/StatusContent/StatusGrid/HullBar
@onready var shields_bar: ProgressBar = $MainSplit/LeftSection/ShipStatus/StatusContent/StatusGrid/ShieldsBar
@onready var power_bar: ProgressBar = $MainSplit/LeftSection/ShipStatus/StatusContent/StatusGrid/PowerBar
@onready var crew_value: Label = $MainSplit/LeftSection/ShipStatus/StatusContent/StatusGrid/CrewValue

@onready var green_alert_btn: Button = $MainSplit/LeftSection/AlertSection/AlertContent/AlertButtons/GreenAlert
@onready var yellow_alert_btn: Button = $MainSplit/LeftSection/AlertSection/AlertContent/AlertButtons/YellowAlert
@onready var red_alert_btn: Button = $MainSplit/LeftSection/AlertSection/AlertContent/AlertButtons/RedAlert
@onready var current_alert: Label = $MainSplit/LeftSection/AlertSection/AlertContent/CurrentAlert

@onready var orders_list: ItemList = $MainSplit/LeftSection/Orders/OrdersContent/OrdersList
@onready var add_order_btn: Button = $MainSplit/LeftSection/Orders/OrdersContent/OrderActions/AddOrder
@onready var clear_orders_btn: Button = $MainSplit/LeftSection/Orders/OrdersContent/OrderActions/ClearOrders

@onready var mission_name: Label = $MainSplit/RightSection/MissionSection/MissionContent/MissionName
@onready var objective_list: ItemList = $MainSplit/RightSection/MissionSection/MissionContent/ObjectiveList

@onready var comm_log: RichTextLabel = $MainSplit/RightSection/CommSection/CommContent/CommLog
@onready var hail_btn: Button = $MainSplit/RightSection/CommSection/CommContent/QuickComm/HailBtn
@onready var broadcast_btn: Button = $MainSplit/RightSection/CommSection/CommContent/QuickComm/BroadcastBtn

@onready var self_destruct_btn: Button = $MainSplit/RightSection/SelfDestruct/SelfDestructContent/SelfDestructBtn
@onready var abort_btn: Button = $MainSplit/RightSection/SelfDestruct/SelfDestructContent/AbortBtn

var _self_destruct_active := false
var _self_destruct_timer := 0.0
var _orders: Array[String] = []


func _ready() -> void:
	_setup_alert_styles()
	_connect_signals()


func _process(delta: float) -> void:
	_update_display()
	
	if _self_destruct_active:
		_self_destruct_timer -= delta
		if _self_destruct_timer <= 0:
			_trigger_self_destruct()


func _setup_alert_styles() -> void:
	# Style alert buttons with appropriate colors
	_style_button(green_alert_btn, Colors.ALERT_GREEN)
	_style_button(yellow_alert_btn, Colors.ALERT_YELLOW)
	_style_button(red_alert_btn, Colors.ALERT_RED)


func _style_button(btn: Button, color: Color) -> void:
	var style := StyleBoxFlat.new()
	style.bg_color = Color(color.r * 0.3, color.g * 0.3, color.b * 0.3, 0.8)
	style.border_color = color
	style.set_border_width_all(2)
	style.set_corner_radius_all(4)
	btn.add_theme_stylebox_override("normal", style)
	
	var pressed := StyleBoxFlat.new()
	pressed.bg_color = Color(color.r * 0.6, color.g * 0.6, color.b * 0.6, 0.9)
	pressed.border_color = color
	pressed.set_border_width_all(3)
	pressed.set_corner_radius_all(4)
	btn.add_theme_stylebox_override("pressed", pressed)


func _connect_signals() -> void:
	GameState.state_updated.connect(_on_state_updated)
	GameState.mission_event.connect(_on_mission_event)
	
	green_alert_btn.pressed.connect(func(): _set_alert("green"))
	yellow_alert_btn.pressed.connect(func(): _set_alert("yellow"))
	red_alert_btn.pressed.connect(func(): _set_alert("red"))
	
	add_order_btn.pressed.connect(_on_add_order)
	clear_orders_btn.pressed.connect(_on_clear_orders)
	
	hail_btn.pressed.connect(_on_hail)
	broadcast_btn.pressed.connect(_on_broadcast)
	
	self_destruct_btn.pressed.connect(_on_self_destruct)
	abort_btn.pressed.connect(_on_abort_self_destruct)


func _update_display() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	
	_update_ship_status(ship)
	_update_alert_status()
	_update_mission_status()


func _update_ship_status(ship: GameState.ShipState) -> void:
	ship_name.text = ship.display_name
	
	# Hull integrity
	hull_bar.value = ship.hull
	hull_bar.modulate = Colors.get_health_color(ship.hull / 100.0)
	
	# Average shields
	var shield_total := 0.0
	for facing in ship.shields:
		shield_total += ship.shields[facing]
	var shield_avg := shield_total / max(ship.shields.size(), 1)
	shields_bar.value = shield_avg
	shields_bar.modulate = Colors.get_shield_color(shield_avg / 100.0)
	
	# Power
	var power_percent := (ship.power_available / ship.power_total) * 100.0 if ship.power_total > 0 else 0
	power_bar.value = power_percent
	power_bar.modulate = Colors.get_power_color(power_percent / 100.0)
	
	# Crew status (based on damage)
	var damage_total := 0.0
	var section_count := 0
	for section in ship.damage_sections:
		damage_total += 100.0 - ship.damage_sections[section].health
		section_count += 1
	
	var avg_damage := damage_total / max(section_count, 1)
	if avg_damage < 10:
		crew_value.text = "NOMINAL"
		crew_value.add_theme_color_override("font_color", Colors.STATUS_ONLINE)
	elif avg_damage < 30:
		crew_value.text = "MINOR CASUALTIES"
		crew_value.add_theme_color_override("font_color", Colors.ALERT_YELLOW)
	elif avg_damage < 60:
		crew_value.text = "MODERATE CASUALTIES"
		crew_value.add_theme_color_override("font_color", Colors.ALERT_ORANGE)
	else:
		crew_value.text = "HEAVY CASUALTIES"
		crew_value.add_theme_color_override("font_color", Colors.ALERT_RED)


func _update_alert_status() -> void:
	var alert := GameState.alert_status
	
	match alert:
		"green":
			current_alert.text = "CONDITION GREEN"
			current_alert.add_theme_color_override("font_color", Colors.ALERT_GREEN)
			green_alert_btn.button_pressed = true
		"yellow":
			current_alert.text = "CONDITION YELLOW"
			current_alert.add_theme_color_override("font_color", Colors.ALERT_YELLOW)
			yellow_alert_btn.button_pressed = true
		"red":
			current_alert.text = "CONDITION RED"
			current_alert.add_theme_color_override("font_color", Colors.ALERT_RED)
			red_alert_btn.button_pressed = true
	
	# Update self-destruct display
	if _self_destruct_active:
		self_destruct_btn.text = "SELF DESTRUCT: %.0f" % _self_destruct_timer
		self_destruct_btn.disabled = true
		abort_btn.disabled = false
	else:
		self_destruct_btn.text = "SELF DESTRUCT"
		self_destruct_btn.disabled = false
		abort_btn.disabled = true


func _update_mission_status() -> void:
	mission_name.text = "Current Mission: %s" % GameState.mission_name
	
	var objectives := GameState.mission_objectives
	objective_list.clear()
	
	for obj in objectives:
		var prefix := "✓ " if obj.get("completed", false) else "○ "
		var idx := objective_list.add_item(prefix + obj.get("description", "Unknown objective"))
		
		if obj.get("completed", false):
			objective_list.set_item_custom_fg_color(idx, Colors.ALERT_GREEN)
		elif obj.get("failed", false):
			objective_list.set_item_custom_fg_color(idx, Colors.ALERT_RED)


func _set_alert(level: String) -> void:
	NetworkClient.send_action("captain", "set_alert", {"level": level})
	_add_comm_log("CAPTAIN", "Set ship to %s alert" % level.to_upper())


func _on_add_order() -> void:
	# In a real implementation, this would open a dialog
	# For now, we'll add a placeholder
	var order := "Standing Order %d" % (_orders.size() + 1)
	_orders.append(order)
	orders_list.add_item(order)
	NetworkClient.send_action("captain", "add_order", {"order": order})


func _on_clear_orders() -> void:
	_orders.clear()
	orders_list.clear()
	NetworkClient.send_action("captain", "clear_orders", {})


func _on_hail() -> void:
	NetworkClient.send_action("comms", "hail", {})
	_add_comm_log("COMMS", "Opening hailing frequencies...")


func _on_broadcast() -> void:
	NetworkClient.send_action("comms", "broadcast", {})
	_add_comm_log("COMMS", "Broadcasting on all frequencies...")


func _on_self_destruct() -> void:
	# Requires confirmation in real implementation
	_self_destruct_active = true
	_self_destruct_timer = 60.0
	NetworkClient.send_action("captain", "self_destruct", {"initiate": true})
	_add_comm_log("COMPUTER", "[color=red]SELF DESTRUCT SEQUENCE INITIATED - 60 SECONDS[/color]")


func _on_abort_self_destruct() -> void:
	_self_destruct_active = false
	_self_destruct_timer = 0.0
	NetworkClient.send_action("captain", "self_destruct", {"initiate": false})
	_add_comm_log("COMPUTER", "[color=green]Self destruct sequence aborted[/color]")


func _trigger_self_destruct() -> void:
	_self_destruct_active = false
	NetworkClient.send_action("captain", "self_destruct_execute", {})
	_add_comm_log("COMPUTER", "[color=red]SELF DESTRUCT COMPLETE[/color]")


func _add_comm_log(source: String, message: String) -> void:
	var time_str := Time.get_time_string_from_system()
	comm_log.append_text("[%s] [b]%s:[/b] %s\n" % [time_str, source, message])


func _on_state_updated() -> void:
	pass  # Updates handled in _process


func _on_mission_event(event: Dictionary) -> void:
	var event_type := event.get("type", "")
	var message := event.get("message", "")
	
	match event_type:
		"objective_complete":
			_add_comm_log("MISSION", "[color=green]Objective completed: %s[/color]" % message)
		"objective_failed":
			_add_comm_log("MISSION", "[color=red]Objective failed: %s[/color]" % message)
		"message":
			_add_comm_log("MISSION", message)
		"incoming_comm":
			_add_comm_log(event.get("source", "UNKNOWN"), message)
