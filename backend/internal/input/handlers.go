package input

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"celestial/internal/ship"
)

// SelfDestructDuration is the arming window before the destruct sequence runs.
// The countdown lives on the server; clients only render server state.
const SelfDestructDuration = 60.0

type repairTeam struct {
	Name     string
	Status   string
	Location string
}

type waypoint struct {
	ID   string
	Name string
	X    float64
	Y    float64
	Z    float64
}

type scanState struct {
	Active   bool
	TargetID string
	Progress float64
	Mode     string
	Duration float64
}

// ---------------------------------------------------------------------------
// Power
// ---------------------------------------------------------------------------

func (ar *ActionRouter) handleToggleBreaker(action *Action) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	d := valueDict(action)
	breaker := dictString(d, "breaker")
	if breaker == "" {
		breaker = dictString(d, "breaker_id")
	}
	if breaker == "" {
		return fmt.Errorf("toggle_breaker requires breaker")
	}
	enabled, ok := dictBool(d, "enabled")
	if !ok {
		// Panels send a bare press; toggling is the intent.
		enabled = !sh.BreakerOn(breaker)
	}
	sh.SetBreaker(breaker, enabled)
	return nil
}

func (ar *ActionRouter) handleRoutePower(action *Action) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	d := valueDict(action)
	target := dictString(d, "system")
	if target == "" {
		target = dictString(d, "target")
	}
	if target == "" {
		return fmt.Errorf("route_power requires system")
	}
	enabled, ok := dictBool(d, "enabled")
	if !ok {
		enabled = true
	}
	sh.SetBreaker(target, enabled)
	return nil
}

// ---------------------------------------------------------------------------
// Damage control
// ---------------------------------------------------------------------------

func (ar *ActionRouter) handleRepair(action *Action) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	d := valueDict(action)
	section, err := normalizeSection(dictString(d, "section"))
	if err != nil {
		return err
	}
	amount := 25.0
	if v, ok := dictFloat(d, "amount"); ok {
		amount = v
	}
	sh.RepairSection(section, amount)
	return nil
}

func (ar *ActionRouter) handleExtinguish(action *Action) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	d := valueDict(action)
	section, err := normalizeSection(dictString(d, "section"))
	if err != nil {
		return err
	}
	sh.ExtinguishFire(section)
	return nil
}

func (ar *ActionRouter) handleSealBreach(action *Action) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	d := valueDict(action)
	section, err := normalizeSection(dictString(d, "section"))
	if err != nil {
		return err
	}
	sh.SealBreach(section)
	return nil
}

// ---------------------------------------------------------------------------
// Repair teams
// ---------------------------------------------------------------------------

func (ar *ActionRouter) handleDeployRepairTeam(action *Action) error {
	d := valueDict(action)
	idx, ok := dictInt(d, "team")
	if !ok {
		idx = 0
	}
	ar.mu.Lock()
	defer ar.mu.Unlock()
	if idx < 0 || idx >= len(ar.teams) {
		return fmt.Errorf("no repair team at index %d", idx)
	}
	team := &ar.teams[idx]
	team.Status = "deployed"
	if loc := dictString(d, "section"); loc != "" {
		if section, err := normalizeSection(loc); err == nil {
			team.Location = section
		}
	}

	// A deployed team repairs continuously while it holds a location.
	ar.teamTimers[idx] = 2.0
	return nil
}

func (ar *ActionRouter) handleRecallRepairTeam(action *Action) error {
	d := valueDict(action)
	idx, ok := dictInt(d, "team")
	if !ok {
		idx = 0
	}
	ar.mu.Lock()
	defer ar.mu.Unlock()
	if idx < 0 || idx >= len(ar.teams) {
		return fmt.Errorf("no repair team at index %d", idx)
	}
	ar.teams[idx].Status = "standing_by"
	ar.teams[idx].Location = ""
	ar.teamTimers[idx] = 0
	return nil
}

func (ar *ActionRouter) handleRepairTeamSection(action *Action) error {
	d := valueDict(action)
	section, err := normalizeSection(dictString(d, "section"))
	if err != nil {
		return err
	}
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	sh.RepairSection(section, 10)
	return nil
}

// Update advances the server-owned timers. Called once per sim tick with the
// simulator lock released.
func (ar *ActionRouter) Update(dt float64) {
	ar.updateRepairTeams(dt)
	ar.updateScan(dt)
	ar.updateSelfDestruct(dt)
	ar.updateAutoFire()
}

func (ar *ActionRouter) updateRepairTeams(dt float64) {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	for i := range ar.teams {
		if ar.teams[i].Status != "deployed" || ar.teams[i].Location == "" {
			continue
		}
		ar.teamTimers[i] -= dt
		if ar.teamTimers[i] > 0 {
			continue
		}
		ar.teamTimers[i] = 2.0
		if sh := ar.getPlayerShip(); sh != nil {
			sh.RepairSection(ar.teams[i].Location, 8)
		}
	}
}

// updateScan advances scan progress on the server; clients only render it.
func (ar *ActionRouter) updateScan(dt float64) {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	if !ar.scan.Active {
		return
	}
	ar.scan.Progress += dt / ar.scan.Duration
	if ar.scan.Progress >= 1.0 {
		ar.scan.Progress = 1.0
		ar.scan.Active = false
	}
}

func (ar *ActionRouter) updateSelfDestruct(dt float64) {
	ar.mu.Lock()
	if ar.selfDestructAt <= 0 {
		ar.mu.Unlock()
		return
	}
	ar.selfDestructAt -= dt
	expired := ar.selfDestructAt <= 0
	if expired {
		ar.selfDestructAt = 0
	}
	ar.mu.Unlock()

	if expired {
		ar.applySelfDestruct()
	}
}

// updateAutoFire launches any torpedo bay that is armed, loaded and locked.
func (ar *ActionRouter) updateAutoFire() {
	ar.mu.Lock()
	enabled := ar.autoFire
	ar.mu.Unlock()
	if !enabled {
		return
	}

	sh := ar.getPlayerShip()
	if sh == nil || sh.GetTargetID() == "" {
		return
	}
	target := ar.simulator.GetShip(sh.GetTargetID())
	if target == nil {
		return
	}

	for _, bay := range sh.WeaponsSnapshot() {
		if bay.Type != "torpedo" || !bay.Armed || !bay.Loaded || !bay.Locked {
			continue
		}
		if bay.Cooldown > 0 || bay.AmmoCount <= 0 {
			continue
		}
		if !sh.InWeaponRange(bay.ID, target.GetPosition()) {
			continue
		}
		if !sh.FireWeapon(bay.ID, target.ID) {
			continue
		}
		ar.simulator.SpawnTorpedo(sh, target, bay.ID)
		return
	}
}

// ---------------------------------------------------------------------------
// Flight
// ---------------------------------------------------------------------------

func (ar *ActionRouter) handleSetThrottle(action *Action) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	d := valueDict(action)
	throttle, ok := dictFloat(d, "throttle")
	if !ok {
		return fmt.Errorf("set_throttle requires throttle")
	}
	sh.SetThrottle(throttle)
	return nil
}

func (ar *ActionRouter) handleSetTurn(action *Action) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	d := valueDict(action)
	rate, ok := dictFloat(d, "rate")
	if !ok {
		pitch, hasPitch := dictFloat(d, "pitch")
		yaw, hasYaw := dictFloat(d, "yaw")
		roll, hasRoll := dictFloat(d, "roll")
		if !hasPitch && !hasYaw && !hasRoll {
			return fmt.Errorf("set_turn requires rate or pitch/yaw/roll")
		}
		sh.ApplyRotation(pitch, yaw, roll)
		return nil
	}
	// Helm turn buttons command yaw only. The old code drove pitch and yaw
	// together, producing a diagonal spin.
	sh.ApplyRotation(0, rate, 0)
	return nil
}

func (ar *ActionRouter) handleSetWaypoint(action *Action) error {
	d := valueDict(action)
	wp := waypoint{
		Name: dictString(d, "name"),
		X:    floatOrZero(d, "x"),
		Y:    floatOrZero(d, "y"),
		Z:    floatOrZero(d, "z"),
	}
	if id := dictString(d, "waypoint_id"); id != "" {
		wp.ID = id
	} else {
		wp.ID = fmt.Sprintf("wp_%d", time.Now().UnixNano())
	}
	if wp.Name == "" {
		wp.Name = wp.ID
	}

	ar.mu.Lock()
	ar.waypoints = append(ar.waypoints, wp)
	ar.mu.Unlock()
	return nil
}

func (ar *ActionRouter) handleClearWaypoint(action *Action) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	ar.waypoints = nil
	return nil
}

func (ar *ActionRouter) handleEngageAutopilot(action *Action) error {
	ar.mu.Lock()
	ar.autopilot = true
	ar.mu.Unlock()
	return nil
}

func (ar *ActionRouter) handleDisengageAutopilot(action *Action) error {
	ar.mu.Lock()
	ar.autopilot = false
	ar.mu.Unlock()
	return nil
}

func (ar *ActionRouter) handleReleaseDocking(action *Action) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	sh.SetDocked(false)
	return nil
}

// ---------------------------------------------------------------------------
// Weapons
// ---------------------------------------------------------------------------

func (ar *ActionRouter) handleSetTarget(action *Action) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	d := valueDict(action)
	targetID := dictString(d, "target_id")
	if targetID == "" {
		return fmt.Errorf("set_target requires target_id")
	}
	if ar.simulator.GetShip(targetID) == nil {
		return fmt.Errorf("target not found: %s", targetID)
	}
	sh.SetTarget(targetID)
	// Acquiring a target locks every armed+loaded tube onto it so the
	// fire buttons (which gate on Locked) work without a separate lock step.
	for _, bay := range sh.WeaponsSnapshot() {
		if bay.Type != "torpedo" || !bay.Armed || !bay.Loaded {
			continue
		}
		_ = sh.MutateWeapon(bay.ID, func(w *ship.Weapon) error {
			w.Locked = true
			return nil
		})
	}
	return nil
}

func (ar *ActionRouter) handleClearTarget(action *Action) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	sh.SetTarget("")
	for _, bay := range sh.WeaponsSnapshot() {
		if bay.Type != "torpedo" || !bay.Locked {
			continue
		}
		_ = sh.MutateWeapon(bay.ID, func(w *ship.Weapon) error {
			w.Locked = false
			return nil
		})
	}
	return nil
}

func (ar *ActionRouter) handleTorpedoArm(action *Action) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	d := valueDict(action)
	bay, err := findTorpedoBay(sh, d)
	if err != nil {
		return err
	}
	enabled := true
	if v, ok := dictBool(d, "armed"); ok {
		enabled = v
	} else if v, ok := dictBool(d, "enabled"); ok {
		enabled = v
	}
	if err := ar.setTorpedoBayFlag(sh, bay.ID, func(w *ship.Weapon) { w.Armed = enabled }); err != nil {
		return err
	}
	// Arming a loaded tube with an active target locks it immediately.
	if enabled && sh.GetTargetID() != "" {
		_ = sh.MutateWeapon(bay.ID, func(w *ship.Weapon) error {
			if w.Loaded {
				w.Locked = true
			}
			return nil
		})
	}
	return nil
}

func (ar *ActionRouter) handleTorpedoLoad(action *Action) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	d := valueDict(action)
	bay, err := findTorpedoBay(sh, d)
	if err != nil {
		return err
	}
	if err := ar.setTorpedoBayFlag(sh, bay.ID, func(w *ship.Weapon) {
		if w.AmmoCount > 0 {
			w.Loaded = true
		}
	}); err != nil {
		return err
	}
	// Loading an armed tube with an active target locks it immediately.
	if sh.GetTargetID() != "" {
		_ = sh.MutateWeapon(bay.ID, func(w *ship.Weapon) error {
			if w.Armed && w.Loaded {
				w.Locked = true
			}
			return nil
		})
	}
	return nil
}

func (ar *ActionRouter) handleTorpedoLock(action *Action) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	d := valueDict(action)
	bay, err := findTorpedoBay(sh, d)
	if err != nil {
		return err
	}
	locked := true
	if v, ok := dictBool(d, "locked"); ok {
		locked = v
	} else if v, ok := dictBool(d, "enabled"); ok {
		locked = v
	}
	return ar.setTorpedoBayFlag(sh, bay.ID, func(w *ship.Weapon) { w.Locked = locked })
}

func (ar *ActionRouter) handleTorpedoFire(action *Action) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	d := valueDict(action)
	bay, err := findTorpedoBay(sh, d)
	if err != nil {
		return err
	}
	if !bay.Loaded {
		return fmt.Errorf("bay %s is empty", bay.ID)
	}
	if !bay.Locked {
		return fmt.Errorf("bay %s has no lock", bay.ID)
	}
	if !bay.Armed {
		return fmt.Errorf("bay %s is not armed", bay.ID)
	}

	targetID := dictString(d, "target_id")
	if targetID == "" {
		targetID = sh.GetTargetID()
	}
	if targetID == "" {
		return fmt.Errorf("no target set")
	}
	target := ar.simulator.GetShip(targetID)
	if target == nil {
		return fmt.Errorf("target not found: %s", targetID)
	}
	if !sh.InWeaponRange(bay.ID, target.GetPosition()) {
		return fmt.Errorf("target out of range")
	}
	if !sh.FireWeapon(bay.ID, targetID) {
		return fmt.Errorf("bay %s cannot fire", bay.ID)
	}
	ar.simulator.SpawnTorpedo(sh, target, bay.ID)
	return nil
}

func (ar *ActionRouter) handleSetAutoFire(action *Action) error {
	d := valueDict(action)
	enabled, _ := dictBool(d, "enabled")
	ar.mu.Lock()
	ar.autoFire = enabled
	ar.mu.Unlock()
	return nil
}

func (ar *ActionRouter) handleSetPhaserEnabled(action *Action) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	d := valueDict(action)
	array, err := findPhaserArray(sh, d)
	if err != nil {
		return err
	}
	enabled := true
	if v, ok := dictBool(d, "enabled"); ok {
		enabled = v
	}
	return sh.MutateWeapon(array.ID, func(w *ship.Weapon) error { w.Enabled = enabled; return nil })
}

func (ar *ActionRouter) handlePhaserFire(action *Action) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	d := valueDict(action)
	array, err := findPhaserArray(sh, d)
	if err != nil {
		return err
	}
	if !array.Enabled {
		return fmt.Errorf("%s is offline", array.ID)
	}
	if array.Cooldown > 0 {
		return fmt.Errorf("%s is recharging", array.ID)
	}

	targetID := dictString(d, "target_id")
	if targetID == "" {
		targetID = sh.GetTargetID()
	}
	if targetID == "" {
		return fmt.Errorf("no target set")
	}
	target := ar.simulator.GetShip(targetID)
	if target == nil {
		return fmt.Errorf("target not found: %s", targetID)
	}
	if !sh.InWeaponRange(array.ID, target.GetPosition()) {
		return fmt.Errorf("target out of range")
	}
	if !sh.FireWeapon(array.ID, targetID) {
		return fmt.Errorf("%s cannot fire", array.ID)
	}

	facing := target.FacingFor(sh.GetPosition())
	target.ApplyTypedDamage(array.Damage, facing, "energy")
	ar.simulator.SpawnPhaserBeam(sh, target)
	return nil
}

// setTorpedoBayFlag applies a flag change to a bay under the ship lock.
func (ar *ActionRouter) setTorpedoBayFlag(sh *ship.Ship, bayID string, fn func(*ship.Weapon)) error {
	return sh.MutateWeapon(bayID, func(w *ship.Weapon) error {
		fn(w)
		return nil
	})
}

// ---------------------------------------------------------------------------
// Captain
// ---------------------------------------------------------------------------

func (ar *ActionRouter) handleSetAlert(action *Action) error {
	d := valueDict(action)
	level := strings.ToLower(dictString(d, "level"))
	switch level {
	case "normal", "green", "yellow", "red":
	default:
		return fmt.Errorf("invalid alert level: %s", level)
	}
	if level == "green" {
		level = "normal"
	}
	ar.simulator.SetAlertLevel(level)
	sh := ar.getPlayerShip()
	if sh != nil {
		sh.SetAlertLevel(level)
	}
	return nil
}

func (ar *ActionRouter) handleIssueOrder(action *Action) error {
	d := valueDict(action)
	order := dictString(d, "order")
	if order == "" {
		return fmt.Errorf("issue_order requires order")
	}
	ar.mu.Lock()
	ar.orders = append(ar.orders, order)
	ar.mu.Unlock()
	return nil
}

func (ar *ActionRouter) handleClearOrders(action *Action) error {
	ar.mu.Lock()
	ar.orders = nil
	ar.mu.Unlock()
	return nil
}

func (ar *ActionRouter) handleAssignOrder(action *Action) error {
	d := valueDict(action)
	order := dictString(d, "order")
	if order == "" {
		return fmt.Errorf("assign_order requires order")
	}
	role := dictString(d, "role")
	if role != "" {
		order = role + ": " + order
	}
	ar.mu.Lock()
	ar.orders = append(ar.orders, order)
	ar.mu.Unlock()
	return nil
}

func (ar *ActionRouter) handleAddLogEntry(action *Action) error {
	d := valueDict(action)
	text := dictString(d, "text")
	if text == "" {
		return fmt.Errorf("add_entry requires text")
	}
	ar.mu.Lock()
	ar.logEntries = append(ar.logEntries, text)
	ar.mu.Unlock()
	return nil
}

func (ar *ActionRouter) handleSelfDestruct(action *Action) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	if ar.selfDestructAt > 0 {
		return fmt.Errorf("self destruct already armed")
	}
	ar.selfDestructAt = SelfDestructDuration
	return nil
}

func (ar *ActionRouter) handleSelfDestructAbort(action *Action) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	ar.selfDestructAt = 0
	return nil
}

func (ar *ActionRouter) applySelfDestruct() {
	sh := ar.getPlayerShip()
	if sh == nil {
		return
	}
	log.Printf("Self destruct sequence executed on %s", sh.ID)
	ar.simulator.RemoveShip(sh.ID)
}

// ---------------------------------------------------------------------------
// Communications
// ---------------------------------------------------------------------------

func (ar *ActionRouter) handleHail(action *Action) error {
	d := valueDict(action)
	targetID := dictString(d, "target_id")
	if targetID == "" {
		return fmt.Errorf("hail requires target_id")
	}
	if ar.simulator.GetShip(targetID) == nil {
		return fmt.Errorf("target not found: %s", targetID)
	}
	if freq, ok := dictFloat(d, "frequency"); ok {
		ar.mu.Lock()
		ar.frequency = freq
		ar.mu.Unlock()
	}
	ar.mu.Lock()
	ar.hailing = true
	ar.hailTarget = targetID
	ar.mu.Unlock()
	return nil
}

func (ar *ActionRouter) handleSendMessage(action *Action) error {
	d := valueDict(action)
	message := dictString(d, "message")
	if message == "" {
		message = dictString(d, "type")
	}
	if message == "" {
		return fmt.Errorf("send_message requires message")
	}
	targetID := dictString(d, "target_id")

	ar.mu.Lock()
	defer ar.mu.Unlock()
	entry := message
	if targetID != "" {
		entry = targetID + ": " + message
	}
	ar.commsLog = append(ar.commsLog, entry)
	if len(ar.commsLog) > 100 {
		ar.commsLog = ar.commsLog[len(ar.commsLog)-100:]
	}
	return nil
}

func (ar *ActionRouter) handleBroadcast(action *Action) error {
	d := valueDict(action)
	message := dictString(d, "message")
	if message == "" {
		message = "shipwide broadcast"
	}
	ar.mu.Lock()
	defer ar.mu.Unlock()
	ar.commsLog = append(ar.commsLog, "ALL CHANNELS: "+message)
	if len(ar.commsLog) > 100 {
		ar.commsLog = ar.commsLog[len(ar.commsLog)-100:]
	}
	return nil
}

func (ar *ActionRouter) handleSetFrequency(action *Action) error {
	d := valueDict(action)
	freq, ok := dictFloat(d, "frequency")
	if !ok {
		return fmt.Errorf("set_frequency requires frequency")
	}
	if freq <= 0 {
		return fmt.Errorf("frequency must be positive")
	}
	ar.mu.Lock()
	ar.frequency = freq
	ar.mu.Unlock()
	return nil
}

// ---------------------------------------------------------------------------
// Sensors
// ---------------------------------------------------------------------------

func (ar *ActionRouter) handleSetSensorMode(action *Action) error {
	d := valueDict(action)
	mode := dictString(d, "mode")
	switch mode {
	case "passive", "active", "deep_scan":
	default:
		return fmt.Errorf("invalid sensor mode: %s", mode)
	}
	ar.mu.Lock()
	ar.scan.Mode = mode
	ar.scan.Active = false
	ar.scan.Progress = 0
	ar.mu.Unlock()
	return nil
}

func (ar *ActionRouter) handleInitiateScan(action *Action) error {
	d := valueDict(action)
	targetID := dictString(d, "target_id")
	if targetID == "" {
		return fmt.Errorf("initiate_scan requires target_id")
	}
	if ar.simulator.GetShip(targetID) == nil {
		return fmt.Errorf("target not found: %s", targetID)
	}
	sh := ar.getPlayerShip()
	if sh != nil && !sh.SubsystemOnline("sensors") {
		return fmt.Errorf("sensors are offline")
	}
	ar.startScan(targetID, 6.0)
	return nil
}

func (ar *ActionRouter) handleDeepScan(action *Action) error {
	d := valueDict(action)
	targetID := dictString(d, "target_id")
	if targetID == "" {
		return fmt.Errorf("deep_scan requires target_id")
	}
	if ar.simulator.GetShip(targetID) == nil {
		return fmt.Errorf("target not found: %s", targetID)
	}
	sh := ar.getPlayerShip()
	if sh != nil && !sh.SubsystemOnline("sensors") {
		return fmt.Errorf("sensors are offline")
	}
	ar.startScan(targetID, 12.0)
	return nil
}

func (ar *ActionRouter) startScan(targetID string, duration float64) {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	ar.scan.Active = true
	ar.scan.TargetID = targetID
	ar.scan.Progress = 0
	ar.scan.Duration = duration
	if ar.scan.Mode != "deep_scan" {
		ar.scan.Mode = "active"
	}
}

func (ar *ActionRouter) handleMarkTarget(action *Action) error {
	d := valueDict(action)
	targetID := dictString(d, "target_id")
	if targetID == "" {
		return fmt.Errorf("mark_target requires target_id")
	}
	sh := ar.getPlayerShip()
	if sh == nil {
		return fmt.Errorf("no player ship")
	}
	sh.SetTarget(targetID)
	return nil
}

func (ar *ActionRouter) handleLaunchProbe(action *Action) error {
	sh := ar.getPlayerShip()
	if sh == nil {
		return fmt.Errorf("no player ship")
	}
	ar.mu.Lock()
	defer ar.mu.Unlock()
	if ar.probes <= 0 {
		return fmt.Errorf("no probes remaining")
	}
	ar.probes--
	return nil
}

// ---------------------------------------------------------------------------
// Shields
// ---------------------------------------------------------------------------

func (ar *ActionRouter) handleRaiseShields(action *Action) error {
	return ar.setShields(action, true)
}

func (ar *ActionRouter) handleLowerShields(action *Action) error {
	return ar.setShields(action, false)
}

func (ar *ActionRouter) setShields(action *Action, enabled bool) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	if enabled && !sh.BreakerOn("shields") {
		return fmt.Errorf("shields breaker is off")
	}
	sh.SetShieldsEnabled(enabled)
	return nil
}

func (ar *ActionRouter) handleShieldFrequency(action *Action) error {
	d := valueDict(action)
	freq, ok := dictFloat(d, "frequency")
	if !ok {
		return fmt.Errorf("set_frequency requires frequency")
	}
	ar.mu.Lock()
	ar.shieldFrequency = freq
	ar.mu.Unlock()
	return nil
}

func (ar *ActionRouter) handleRotateShieldFrequency(action *Action) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	ar.shieldFrequency = math.Mod(math.Floor(ar.shieldFrequency)+1, 100)
	return nil
}

// ---------------------------------------------------------------------------
// Transporter
// ---------------------------------------------------------------------------

func (ar *ActionRouter) handleTransporterBeamUp(action *Action) error {
	return ar.setTransporter(action, true, false)
}

func (ar *ActionRouter) handleTransporterBeamDown(action *Action) error {
	return ar.setTransporter(action, false, false)
}

func (ar *ActionRouter) handleTransporterEmergency(action *Action) error {
	return ar.setTransporter(action, true, true)
}

func (ar *ActionRouter) setTransporter(action *Action, active, emergency bool) error {
	sh, err := ar.playerShip(action)
	if err != nil {
		return err
	}
	if active && !sh.BreakerOn("life_support") {
		return fmt.Errorf("life support breaker is off")
	}
	ar.mu.Lock()
	defer ar.mu.Unlock()
	ar.transporterActive = active
	ar.transporterEmergency = emergency
	return nil
}

// ---------------------------------------------------------------------------
// State accessors used by the protocol layer
// ---------------------------------------------------------------------------

func (ar *ActionRouter) ScanState() (active bool, targetID string, progress float64, mode string) {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	return ar.scan.Active, ar.scan.TargetID, ar.scan.Progress, ar.scan.Mode
}

func (ar *ActionRouter) CommState() (hailing bool, target string, frequency float64) {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	return ar.hailing, ar.hailTarget, ar.frequency
}

func (ar *ActionRouter) ShieldFrequency() float64 {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	return ar.shieldFrequency
}

func (ar *ActionRouter) TransporterState() (active bool, emergency bool) {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	return ar.transporterActive, ar.transporterEmergency
}

func (ar *ActionRouter) SelfDestructRemaining() float64 {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	return ar.selfDestructAt
}

func (ar *ActionRouter) Orders() []string {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	out := make([]string, len(ar.orders))
	copy(out, ar.orders)
	return out
}

func (ar *ActionRouter) CommsLog() []string {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	out := make([]string, len(ar.commsLog))
	copy(out, ar.commsLog)
	return out
}

func (ar *ActionRouter) LogEntries() []string {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	out := make([]string, len(ar.logEntries))
	copy(out, ar.logEntries)
	return out
}

func (ar *ActionRouter) Autopilot() bool {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	return ar.autopilot
}

func (ar *ActionRouter) AutoFire() bool {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	return ar.autoFire
}

func (ar *ActionRouter) Waypoints() []waypoint {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	out := make([]waypoint, len(ar.waypoints))
	copy(out, ar.waypoints)
	return out
}

func (ar *ActionRouter) Probes() int {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	return ar.probes
}

func (ar *ActionRouter) RepairTeams() []map[string]string {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	out := make([]map[string]string, 0, len(ar.teams))
	for _, t := range ar.teams {
		out = append(out, map[string]string{
			"name": t.Name, "status": t.Status, "location": t.Location,
		})
	}
	return out
}

func floatOrZero(d map[string]interface{}, key string) float64 {
	v, _ := dictFloat(d, key)
	return v
}
