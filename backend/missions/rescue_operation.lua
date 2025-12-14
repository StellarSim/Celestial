-- Mission: Rescue Operation
-- Description: Respond to distress call and rescue stranded vessel

mission = {
    name = "Rescue Operation",
    description = "Respond to distress call from merchant vessel under attack"
}

local player_ship = "player_1"
local merchant_ship = "merchant_1"
local rescue_complete = false
local merchant_alive = true
local escort_active = false
local enemies_remaining = 4

function on_start()
    log("Mission started: Rescue Operation")
    
    spawn_ship(player_ship, "player_cruiser", "USS Celestial", true, {x=0, y=0, z=0})
    
    spawn_ship(merchant_ship, "enemy_frigate", "Merchant Vessel Aurora", false, {x=10000, y=500, z=-5000})
    
    set_objective("respond", "Respond to distress call")
    set_objective("defend", "Defend the merchant vessel")
    set_objective("eliminate", "Eliminate all hostiles (0/4)")
    set_objective("escort", "Escort merchant to safety")
    
    log("Distress call received from merchant vessel Aurora")
    log("Pirates attacking! Respond immediately!")
    
    spawn_initial_enemies()
end

function on_event(event_name, params)
    if event_name == "area_reached" then
        local area = params.area
        
        if area == "merchant_location" then
            log("Arrived at merchant vessel location")
            complete_objective("respond")
            
            log("Merchant vessel: 'Thank you for responding! We're under heavy fire!'")
        elseif area == "safe_zone" and escort_active then
            log("Safe zone reached")
            complete_objective("escort")
            mission_win()
        end
    end
    
    if event_name == "ship_destroyed" then
        local ship_id = params.ship_id
        
        if ship_id == merchant_ship then
            merchant_alive = false
            mission_lose("Merchant vessel destroyed")
        elseif ship_id == player_ship then
            mission_lose("Player ship destroyed")
        elseif string.find(ship_id, "pirate_") then
            enemies_remaining = enemies_remaining - 1
            log("Pirate destroyed. Remaining: " .. enemies_remaining)
            
            set_objective("eliminate", "Eliminate all hostiles (" .. (4 - enemies_remaining) .. "/4)")
            
            if enemies_remaining == 2 then
                log("Pirate reinforcements inbound!")
                spawn_reinforcements()
            end
            
            if enemies_remaining == 0 then
                complete_objective("eliminate")
                complete_objective("defend")
                start_escort()
            end
        end
    end
    
    if event_name == "merchant_damaged" then
        local health = params.health
        
        if health < 30 then
            log("WARNING: Merchant vessel critical! Hull at " .. health .. "%")
        elseif health < 60 then
            log("Merchant vessel taking heavy damage! Hull at " .. health .. "%")
        end
    end
end

function spawn_initial_enemies()
    log("Pirate raiders detected attacking merchant vessel")
    
    spawn_ship("pirate_1", "enemy_frigate", "Pirate Raider", false, {x=10500, y=300, z=-4800})
    spawn_ship("pirate_2", "enemy_frigate", "Pirate Raider", false, {x=10200, y=700, z=-5200})
    
    damage_ship(merchant_ship, 150, "forward")
end

function spawn_reinforcements()
    log("Pirate reinforcements detected!")
    
    spawn_ship("pirate_3", "enemy_frigate", "Pirate Gunship", false, {x=11000, y=0, z=-5500})
    spawn_ship("pirate_4", "enemy_frigate", "Pirate Gunship", false, {x=11000, y=0, z=-4500})
end

function start_escort()
    log("All hostiles eliminated")
    log("Merchant vessel: 'We're clear! Please escort us to the safe zone.'")
    
    escort_active = true
    
    spawn_object("safe_zone", "waypoint", {x=-8000, y=0, z=3000})
    log("Escort merchant vessel to safe zone coordinates")
end

function check_merchant_status()
    if merchant_alive and not rescue_complete then
        log("Merchant vessel status nominal")
    end
end
