-- Mission: Border Patrol
--
-- The crew's choices drive the outcome. Hailing, scanning and fighting all
-- change real sim state, and the mission reacts to that state rather than
-- counting kills.

mission = {
    name = "Border Patrol",
    description = "Patrol the gamma-7 border and decide what to do about unknown contacts."
}

local player_id = nil
local contacts_hailed = false
local contacts_scanned = false
local contacts_engaged = false
local patrol_done = false

local WAYPOINT_ALPHA = "waypoint_alpha"
local WAYPOINT_BETA = "waypoint_beta"

local RAIDERS = {"raider_1", "raider_2"}


function on_start()
    spawn_ship("player_1", "player_cruiser", "USS Endeavour", true, {x = 0, y = 0, z = 0})
    player_id = "player_1"

    set_objective("patrol_alpha", "Proceed to the gamma-7 border waypoint")
    set_objective("assess", "Identify the contacts: hail them or scan them")
    set_objective("decide", "Decide what to do about the contacts")

    spawn_object(WAYPOINT_ALPHA, "waypoint", {x = 5000, y = 0, z = 2000})
    spawn_object(WAYPOINT_BETA, "waypoint", {x = 8000, y = 1000, z = -3000})

    log("Border patrol begins. Two patrol waypoints marked on sensors.")
end


function on_event(event_name, params)
    if event_name == "waypoint_reached" then
        on_waypoint(params.waypoint)

    elseif event_name == "crew_action" then
        on_crew_action(params)

    elseif event_name == "damage_critical" then
        if params.ship_id == player_id then
            log("HULL CRITICAL. Damage control has seconds, not minutes.")
            set_objective("survive", "Survive long enough to withdraw")
        end

    elseif event_name == "ship_destroyed" then
        on_ship_destroyed(params.ship_id)
    end
end


function on_waypoint(waypoint_id)
    if waypoint_id == WAYPOINT_ALPHA then
        if patrol_done then
            return
        end
        patrol_done = true
        complete_objective("patrol_alpha")
        log("Border waypoint reached. Two unverified contacts holding station nearby.")

        spawn_ship("raider_1", "enemy_frigate", "Unidentified Contact One", false, {x = 5500, y = 200, z = 2100})
        spawn_ship("raider_2", "enemy_frigate", "Unidentified Contact Two", false, {x = 5200, y = -100, z = 2300})

    elseif waypoint_id == WAYPOINT_BETA then
        if not patrol_done then
            return
        end
        log("Patrol route complete.")
    end
end


-- The crew's choice changes which branch of the mission runs.
function on_crew_action(params)
    local action = params.action
    if action == "hail" then
        contacts_hailed = true
        log("Hailing the contacts. Awaiting reply.")

    elseif action == "scan" then
        if not contacts_scanned then
            contacts_scanned = true
            complete_objective("assess")
            log("Sensor sweep complete. Contacts are running dark, weapons hot.")
            log("They are raiders. Relay has a firing solution warning.")
            set_objective("decide", "Contacts are hostile: drive them off or hold fire")
        end

    elseif action == "fire" then
        contacts_engaged = true
        log("Weapons free. Engaging.")
    end
end


function on_ship_destroyed(ship_id)
    if ship_id == player_id then
        mission_lose("Endeavour destroyed before the patrol was resolved.")
        return
    end

    for _, raider in ipairs(RAIDERS) do
        if raider == ship_id then
            log("Hostile contact destroyed.")
            return
        end
    end
end


-- The GM decides the ending around whatever the crew actually did.
function resolve_outcome()
    if not contacts_scanned then
        complete_objective("decide")
        mission_win("Patrol completed. The contacts were never identified.")
        return
    end

    if contacts_hailed and not contacts_engaged then
        log("The raiders break off and withdraw from the border.")
        complete_objective("decide")
        mission_win("Contacts identified and deterred without firing.")
        return
    end

    if contacts_engaged then
        log("Border secure. Damage control has its own work now.")
        complete_objective("decide")
        mission_win("Patrol resolved by force of arms.")
        return
    end

    set_objective("decide", "Contacts identified: decide how to resolve the contact")
    log("The contacts have been identified. The decision is the crew's to make.")
end