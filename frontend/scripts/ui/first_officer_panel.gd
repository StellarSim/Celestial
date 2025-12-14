extends Control
## First Officer panel - crew management, status overview, and captain's log.

const DEPARTMENTS := ["Bridge", "Engineering", "Security", "Medical", "Science", "Operations"]

@onready var total_crew: Label = $MainSplit/LeftSection/CrewSection/CrewContent/CrewSummary/TotalValue
@onready var active_crew: Label = $MainSplit/LeftSection/CrewSection/CrewContent/CrewSummary/ActiveValue
@onready var injured_crew: Label = $MainSplit/LeftSection/CrewSection/CrewContent/CrewSummary/InjuredValue
@onready var casualties: Label = $MainSplit/LeftSection/CrewSection/CrewContent/CrewSummary/CasualtiesValue
@onready var department_list: ItemList = $MainSplit/LeftSection/CrewSection/CrewContent/DepartmentList

@onready var team_list: VBoxContainer = $MainSplit/LeftSection/RepairTeams/RepairContent/TeamList

@onready var overall_status: Label = $MainSplit/RightSection/StatusSection/StatusContent/OverallStatus
@onready var hull_value: Label = $MainSplit/RightSection/StatusSection/StatusContent/StatusSummary/HullValue
@onready var shields_value: Label = $MainSplit/RightSection/StatusSection/StatusContent/StatusSummary/ShieldsValue
@onready var power_value: Label = $MainSplit/RightSection/StatusSection/StatusContent/StatusSummary/PowerValue
@onready var ammo_value: Label = $MainSplit/RightSection/StatusSection/StatusContent/StatusSummary/AmmoValue

@onready var damage_list: ItemList = $MainSplit/RightSection/DamageReport/DamageContent/DamageList
@onready var log_text: RichTextLabel = $MainSplit/RightSection/LogSection/LogContent/LogText
@onready var add_entry_btn: Button = $MainSplit/RightSection/LogSection/LogContent/LogActions/AddEntry
@onready var export_btn: Button = $MainSplit/RightSection/LogSection/LogContent/LogActions/ExportLog

var _total_crew := 150
var _injured := 0
var _dead := 0
var _repair_teams: Array[Dictionary] = [
	{"name": "Alpha Team", "status": "standing_by", "location": ""},
	{"name": "Beta Team", "status": "standing_by", "location": ""},
	{"name": "Gamma Team", "status": "standing_by", "location": ""}
]
var _log_entries: Array[Dictionary] = []


func _ready() -> void:
	_setup_departments()
	_setup_repair_teams()
	_connect_signals()


func _process(_delta: float) -> void:
	_update_display()


func _setup_departments() -> void:
	department_list.clear()
	for dept in DEPARTMENTS:
		department_list.add_item("%s: 25 crew" % dept)


func _setup_repair_teams() -> void:
	for i in range(team_list.get_child_count()):
		var team_row := team_list.get_child(i) as HBoxContainer
		var deploy_btn := team_row.get_child(2) as Button
		deploy_btn.pressed.connect(_on_deploy_team.bind(i))


func _connect_signals() -> void:
	GameState.state_updated.connect(_on_state_updated)
	
	add_entry_btn.pressed.connect(_on_add_log_entry)
	export_btn.pressed.connect(_on_export_log)


func _update_display() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	
	_update_crew_status(ship)
	_update_repair_teams()
	_update_ship_status(ship)
	_update_damage_report(ship)


func _update_crew_status(ship: GameState.ShipState) -> void:
	# Calculate casualties from damage
	var total_damage := 0.0
	var section_count := 0
	
	for section in ship.damage_sections:
		total_damage += 100.0 - ship.damage_sections[section].health
		section_count += 1
	
	var avg_damage: float = total_damage / max(section_count, 1)
	
	# Estimate casualties based on damage
	_injured = int(avg_damage * 0.5)
	_dead = int(avg_damage * 0.2)
	var active := _total_crew - _injured - _dead
	
	total_crew.text = str(_total_crew)
	active_crew.text = str(active)
	injured_crew.text = str(_injured)
	
	if _injured > 0:
		injured_crew.add_theme_color_override("font_color", Colors.ALERT_YELLOW)
	else:
		injured_crew.add_theme_color_override("font_color", Colors.TEXT_NORMAL)
	
	casualties.text = str(_dead)
	if _dead > 0:
		casualties.add_theme_color_override("font_color", Colors.ALERT_RED)
	else:
		casualties.add_theme_color_override("font_color", Colors.TEXT_NORMAL)


func _update_repair_teams() -> void:
	for i in range(min(team_list.get_child_count(), _repair_teams.size())):
		var team := _repair_teams[i]
		var row := team_list.get_child(i) as HBoxContainer
		var status_label := row.get_child(1) as Label
		var deploy_btn := row.get_child(2) as Button
		
		match team.status:
			"standing_by":
				status_label.text = "STANDING BY"
				status_label.add_theme_color_override("font_color", Colors.STATUS_ONLINE)
				deploy_btn.text = "DEPLOY"
				deploy_btn.disabled = false
			"deployed":
				status_label.text = "DEPLOYED: %s" % team.location
				status_label.add_theme_color_override("font_color", Colors.ALERT_YELLOW)
				deploy_btn.text = "RECALL"
				deploy_btn.disabled = false
			"repairing":
				status_label.text = "REPAIRING: %s" % team.location
				status_label.add_theme_color_override("font_color", Colors.PRIMARY)
				deploy_btn.text = "RECALL"
				deploy_btn.disabled = false


func _update_ship_status(ship: GameState.ShipState) -> void:
	# Overall status assessment
	var hull: float = ship.hull_integrity
	var shield_avg: float = 0.0
	var facing_count: int = 0
	for facing in ship.shield_facings:
		var facing_val: float = ship.shield_facings[facing]
		shield_avg += facing_val
		facing_count += 1
	shield_avg = shield_avg / max(facing_count, 1)
	
	var power_percent: float = (ship.power_available / ship.power_total * 100.0) if ship.power_total > 0 else 0.0
	
	# Determine overall status
	if hull < 25 or power_percent < 20:
		overall_status.text = "CRITICAL"
		overall_status.add_theme_color_override("font_color", Colors.ALERT_RED)
	elif hull < 50 or power_percent < 50 or shield_avg < 30:
		overall_status.text = "DAMAGED"
		overall_status.add_theme_color_override("font_color", Colors.ALERT_ORANGE)
	elif hull < 75 or shield_avg < 60:
		overall_status.text = "DEGRADED"
		overall_status.add_theme_color_override("font_color", Colors.ALERT_YELLOW)
	else:
		overall_status.text = "OPERATIONAL"
		overall_status.add_theme_color_override("font_color", Colors.STATUS_ONLINE)
	
	# Update individual values
	hull_value.text = "%.0f%%" % hull
	hull_value.add_theme_color_override("font_color", Colors.get_health_color(hull / 100.0))
	
	shields_value.text = "%.0f%%" % shield_avg
	shields_value.add_theme_color_override("font_color", Colors.get_shield_color(shield_avg / 100.0))
	
	power_value.text = "%.0f%%" % power_percent
	power_value.add_theme_color_override("font_color", Colors.get_power_color(power_percent / 100.0))
	
	# Ammunition status
	var torpedo_count := 0
	for torp_type in ship.weapons.torpedo_inventory:
		torpedo_count += ship.weapons.torpedo_inventory[torp_type]
	
	if torpedo_count > 15:
		ammo_value.text = "FULL"
		ammo_value.add_theme_color_override("font_color", Colors.STATUS_ONLINE)
	elif torpedo_count > 5:
		ammo_value.text = "LOW (%d)" % torpedo_count
		ammo_value.add_theme_color_override("font_color", Colors.ALERT_YELLOW)
	else:
		ammo_value.text = "CRITICAL (%d)" % torpedo_count
		ammo_value.add_theme_color_override("font_color", Colors.ALERT_RED)


func _update_damage_report(ship: GameState.ShipState) -> void:
	damage_list.clear()
	
	for section_name in ship.damage_sections:
		var section = ship.damage_sections[section_name]
		
		if section.health < 100:
			var idx := damage_list.add_item("%s: %.0f%% integrity" % [section_name.capitalize(), section.health])
			damage_list.set_item_custom_fg_color(idx, Colors.get_health_color(section.health / 100.0))
		
		if section.fires > 0:
			var idx := damage_list.add_item("  🔥 %d fire(s) in %s" % [section.fires, section_name.capitalize()])
			damage_list.set_item_custom_fg_color(idx, Colors.ALERT_RED)
		
		if section.breaches > 0:
			var idx := damage_list.add_item("  ⚠ %d breach(es) in %s" % [section.breaches, section_name.capitalize()])
			damage_list.set_item_custom_fg_color(idx, Colors.ALERT_ORANGE)
	
	if damage_list.item_count == 0:
		damage_list.add_item("No damage to report")


func _on_deploy_team(team_idx: int) -> void:
	var team := _repair_teams[team_idx]
	
	if team.status == "standing_by":
		# Find section with most damage
		var ship := GameState.get_player_ship()
		if ship == null:
			return
		
		var worst_section := ""
		var worst_health := 100.0
		
		for section_name in ship.damage_sections:
			var section = ship.damage_sections[section_name]
			if section.health < worst_health:
				worst_health = section.health
				worst_section = section_name
		
		if not worst_section.is_empty():
			team.status = "deployed"
			team.location = worst_section.capitalize()
			NetworkClient.send_action("xo", "deploy_team", {
				"team": team_idx,
				"section": worst_section
			})
	else:
		team.status = "standing_by"
		team.location = ""
		NetworkClient.send_action("xo", "recall_team", {"team": team_idx})


func _on_add_log_entry() -> void:
	var ship := GameState.get_player_ship()
	var stardate := "%.1f" % (Time.get_unix_time_from_system() / 86400.0 + 41000)
	
	var entry := {
		"stardate": stardate,
		"time": Time.get_time_string_from_system(),
		"text": "Captain's log, supplemental."
	}
	_log_entries.append(entry)
	
	log_text.append_text("[b]Stardate %s[/b]\n%s\n\n" % [stardate, entry.text])


func _on_export_log() -> void:
	# In a real implementation, this would save to a file
	pass


func _on_state_updated() -> void:
	pass  # Updates handled in _process
