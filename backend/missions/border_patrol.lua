-- Mission: Border Patrol
-- Description: Patrol the neutral zone and investigate unknown contacts

mission = {
    name = "Border Patrol",
    description = "Patrol sector gamma-7 and investigate any unknown contacts"
}

local player_ship = "player_1"
local patrol_complete = false
local enemies_destroyed = 0
local enemies_required = 3

function on_start()
    log("Mission started: Border Patrol")
    
    spawn_ship(player_ship, "player_cruiser", "USS Celestial", true, {x=0, y=0, z=0})
    
    set_objective("patrol_1", "Navigate to waypoint Alpha")
    set_objective("patrol_2", "Navigate to waypoint Beta")
    set_objective("investigate", "Investigate unknown contacts")
    set_objective("eliminate_threats", "Eliminate hostile threats (0/3)")
    
    spawn_object("waypoint_alpha", "waypoint", {x=5000, y=0, z=2000})
    spawn_object("waypoint_beta", "waypoint", {x=8000, y=1000, z=-3000})
    
    log("Objectives set. Proceed to waypoint Alpha.")
end

function on_event(event_name, params)
    if event_name == "waypoint_reached" then
        local waypoint = params.waypoint
        
        if waypoint == "alpha" then
            log("Waypoint Alpha reached")
            complete_objective("patrol_1")
            spawn_enemy_patrol()
        elseif waypoint == "beta" then
            log("Waypoint Beta reached")
            complete_objective("patrol_2")
            spawn_heavy_enemy()
        end
    end
    
    if event_name == "ship_destroyed" then
        local ship_id = params.ship_id
        
        if string.find(ship_id, "enemy_") then
            enemies_destroyed = enemies_destroyed + 1
            log("Enemy destroyed. Count: " .. enemies_destroyed .. "/" .. enemies_required)
            
            set_objective("eliminate_threats", "Eliminate hostile threats (" .. enemies_destroyed .. "/3)")
            
            if enemies_destroyed >= enemies_required then
                complete_objective("eliminate_threats")
                complete_objective("investigate")
                mission_win()
            end
        elseif ship_id == player_ship then
            mission_lose("Player ship destroyed")
        end
    end
    
    if event_name == "damage_critical" then
        log("WARNING: Critical damage sustained!")
    end
end

function spawn_enemy_patrol()
    log("Unknown contacts detected near waypoint Alpha")
    
    spawn_ship("enemy_1", "enemy_frigate", "Hostile Frigate", false, {x=5500, y=200, z=2100})
    spawn_ship("enemy_2", "enemy_frigate", "Hostile Frigate", false, {x=5200, y=-100, z=2300})
    
    log("Two hostile frigates approaching!")
end

function spawn_heavy_enemy()
    log("Large contact detected near waypoint Beta")
    
    spawn_ship("enemy_3", "enemy_frigate", "Hostile Heavy Frigate", false, {x=8200, y=1000, z=-2800})
    
    log("Hostile heavy frigate engaging!")
end
