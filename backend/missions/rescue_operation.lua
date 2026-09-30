-- Mission: Rescue Operation
--
-- The merchant vessel survives or it does not, based on what the crew does.
-- The ending follows the state of the simulation, not a counter.

mission = {
    name = "Rescue Operation",
    description = "Answer a distress call and get the crew of the Aurora to safety."
}

local player_id = nil
local merchant_id = "aurora"
local hauntress = "hauntress_1"

local DOCK_RANGE = 300.0


function on_start()
    spawn_ship("player_1", "player_cruiser", "USS Endeavour", true, {x = 0, y = 0, z = 0})
    player_id = "player_1"

    spawn_ship(merchant_id, "enemy_frigate", "Merchant Vessel Aurora", false, {x = 10000, y = 500, z = -5000})
    spawn_ship(hauntress, "enemy_frigate", "Pirate Hauntress", false, {x = 10400, y = 400, z = -4900})

    damage_ship(merchant_id, 220, "aft")

    set_objective("respond", "Reach the Aurora")
    set_objective("protect", "Keep the Aurora alive")

    log("Distress call from the Aurora. A hauntress is closing on her stern.")
    log("No time to find out who they are.")
end


function on_event(event_name, params)
    if event_name == "crew_action" then
        on_crew_action(params)

    elseif event_name == "damage_critical" then
        on_critical(params)

    elseif event_name == "ship_destroyed" then
        on_ship_destroyed(params.ship_id)
    end
end


function on_crew_action(params)
    local action = params.action
    local merchant_health = ship_health(merchant_id)

    if action == "hail" then
        log("Aurora: 'We are hit and losing pressure. Please, get her off!'")
        set_objective("protect", "Get a damage team to the Aurora while the raider pressures her")

    elseif action == "repair_team" then
        log("Repair team away for the Aurora.")
        set_objective("protect", "Repair team working: hold the hauntress off")

    elseif action == "dock" then
        if merchant_health <= 0 then
            log("Nothing left to dock with.")
            return
        end
        log("Docking clamps engaged with the Aurora.")
        complete_objective("respond")
        complete_objective("protect")
        mission_win("Aurora crew transferred. The hauntress broke off.")
    end
end


function on_critical(params)
    if params.ship_id == player_id then
        log("Endeavour is critical. The rescue is going badly.")
        set_objective("protect", "Survive: the Aurora still needs you")

    elseif params.ship_id == merchant_id then
        log("Aurora hull failing. The repair team has minutes.")
        set_objective("protect", "Aurora failing: get pressure back on her now")

    elseif params.ship_id == hauntress then
        log("Hauntress is withdrawing under fire.")
    end
end


function on_ship_destroyed(ship_id)
    if ship_id == player_id then
        mission_lose("Endeavour destroyed. No one came for the Aurora.")
        return
    end

    if ship_id == merchant_id then
        mission_lose("Aurora destroyed before the crew could be transferred.")
        return
    end

    if ship_id == hauntress then
        complete_objective("respond")
        set_objective("protect", "Aurora is clear: dock and take the crew aboard")
        log("The hauntress is gone. The Aurora is not going anywhere without you.")
    end
end