package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

const RedisIncursionKey = "incursions"

func (server *Server) SetupIncursions() error {
	incursionsCmd := server.Redis.Get(RedisIncursionKey)
	if incursionsCmd.Err() != nil {
		// Essentially no stashed incursions
		return incursionsCmd.Err()
	}

	var savedIncursions []*EsiIncursion

	err := json.Unmarshal([]byte(incursionsCmd.Val()), &savedIncursions)

	if err != nil {
		log.Printf("[ERROR] Unable to parse saved incursion json in redis! JSON: %v Error: %v", incursionsCmd.Val(), err)
		return err
	}

	// Resolve all the names
	// Note: It would be possible to have all the names serialized out with json, but that might be a discussion for later
	server.PopulateIncursionData(savedIncursions)

	return nil
}

func (server *Server) checkIncursions() {
	incursions, new := server.GetIncursions()

	// If we got a cached version, don't even bother checking...
	if !new {
		return
	}

	// Otherwise, lets compare our scheduler incursions to the returned incursions
	// Note: Special case, the first time this runs we will have no cached incursions, so lets just skip that run to hydrate the cache

	if len(lastIncursions) <= 0 {
		lastIncursions = incursions // hydrate
		return
	}

	newIncursions := make([]*EsiIncursion, 0)
	changedIncursions := make([]*EsiIncursion, 0)
	deadIncursions = make([]*EsiIncursion, 0)

	// Lets just walk through the array and attempt to compare each other incursion
	for _, inc := range incursions {
		foundExisting := false
		for _, existing := range lastIncursions {
			if inc.StagingSolarSystemId == existing.StagingSolarSystemId {
				// Check if state also changed
				if inc.State != existing.State {
					// Set the "previous state" temp variable so we can use later
					changedIncursions = append(changedIncursions, inc)
				}

				foundExisting = true
				break
			}
		}

		if !foundExisting {
			newIncursions = append(newIncursions, inc)
		}
	}

	for _, existing := range lastIncursions {
		foundExisting := false
		for _, new := range incursions {
			if existing.StagingSolarSystemId == new.StagingSolarSystemId {
				foundExisting = true
				break
			}
		}

		// It's still alive, SKIP
		if foundExisting {
			continue
		}

		deadIncursions = append(deadIncursions, existing)
	}

	var buffer = bytes.NewBufferString("")

	for _, changed := range changedIncursions {
		server.GetChangedIncursionMessage(changed, buffer)
	}

	for _, new := range newIncursions {
		server.GetNewIncursionMessage(new, buffer)
	}

	for _, dead := range deadIncursions {
		server.GetDespawnedIncursionMessage(dead, buffer)
	}

	// Did any new info come through?
	if buffer.Len() > 0 {
		msg := buffer.String()

		if len(msg) > 0 {
			server.BroadcastMessage(msg)
		} else {
			log.Printf("All remains quiet...")
		}
	}

	// Cache of the last result
	lastIncursions = incursions
}

// TODO: It feels like this method doesn't belong here since the struct isn't here
func (server *Server) GetConstellationForIncursion(incursion *EsiIncursion) *EsiConstellation {
	return server.GetConstellation(incursion.ConstellationId)
}

func (server *Server) GetNewIncursionMessage(incursion *EsiIncursion, buffer *bytes.Buffer) {
	constellation := server.GetConstellationForIncursion(incursion)
	dotlan := fmt.Sprintf("http://evemaps.dotlan.net/map/%v/%v", constellation.RegionName, constellation.Name)

	dotlan = strings.Replace(dotlan, " ", "_", -1)

	// Filter
	if incursion.StagingSystem.SecurityStatus <= server.Config.SecurityStatusThreshold {
		buffer.WriteString(fmt.Sprintf("New Incursion detected in %v {%.1v} {%v - %v} - %v jumps from staging - Dotlan: %v\n", incursion.StagingSystem.Name, incursion.StagingSystem.SecurityStatus, incursion.ConsellationName, constellation.RegionName, len(incursion.Route)-1, dotlan))
	}
}

func (server *Server) GetDefaultIncurionsMessage(incursion *EsiIncursion, buffer *bytes.Buffer) {
	constellation := server.GetConstellationForIncursion(incursion)
	dotlan := fmt.Sprintf("http://evemaps.dotlan.net/map/%v/%v", constellation.RegionName, constellation.Name)
	dotlan = strings.Replace(dotlan, " ", "_", -1)

	if incursion.StagingSystem.SecurityStatus <= server.Config.SecurityStatusThreshold {
		buffer.WriteString(fmt.Sprintf("%v {%.1v} {%v - %v} Influence: %.3v%% - Status %v- %v jumps from staging - Dotlan: %v\n", incursion.StagingSystem.Name, incursion.StagingSystem.SecurityStatus, incursion.ConsellationName, constellation.RegionName, incursion.Influence*100, incursion.State, len(incursion.Route)-1, dotlan))
	}
}

func (server *Server) GetChangedIncursionMessage(incursion *EsiIncursion, buffer *bytes.Buffer) {
	constellation := server.GetConstellationForIncursion(incursion)
	dotlan := fmt.Sprintf("http://evemaps.dotlan.net/map/%v/%v", constellation.RegionName, constellation.Name)
	dotlan = strings.Replace(dotlan, " ", "_", -1)

	if incursion.StagingSystem.SecurityStatus <= server.Config.SecurityStatusThreshold {
		buffer.WriteString(fmt.Sprintf("Incursion in %v {%.1v} {%v - %v} Changed status to - Status %v - %v jumps from staging - Dotlan: %v\n", incursion.StagingSystem.Name, incursion.StagingSystem.SecurityStatus, incursion.ConsellationName, constellation.RegionName, incursion.State, len(incursion.Route)-1, dotlan))
	}
}

func (server *Server) GetDespawnedIncursionMessage(incursion *EsiIncursion, buffer *bytes.Buffer) {
	constellation := server.GetConstellationForIncursion(incursion)

	if incursion.StagingSystem.SecurityStatus <= server.Config.SecurityStatusThreshold {
		buffer.WriteString(fmt.Sprintf("Incursion in %v {%.1v} {%v - %v} Despawned", incursion.StagingSystem.Name, incursion.StagingSystem.SecurityStatus, incursion.ConsellationName, constellation.RegionName))
	}
}
