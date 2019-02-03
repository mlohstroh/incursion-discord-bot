package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

const EsiHostname = "https://esi.evetech.net"

type EsiStatus struct {
	Players       int       `json:"players"`
	ServerVersion string    `json:"server_version"`
	StartTime     time.Time `json:"start_time"`
}

type EsiIncursion struct {
	ConstellationId      int `json:"constellation_id"`
	ConsellationName     string
	FactionId            int `json:"faction_id"`
	StagingSolarSystemId int `json:"staging_solar_system_id"`
	StagingSystem        *EsiSystem
	HasBoss              bool    `json:"has_boss"`
	InfestedSolarSystems []int   `json:"infested_solar_systems"`
	Influence            float32 `json:"influence"`
	State                string  `json:"state"`
	Type                 string  `json:"type"`
	Route                []int   `json:"route"`
	DeathTime            int     // temp for dead incursions
}

type NameRequest struct {
	Ids []int `json:"ids"`
}

type EsiName struct {
	Category string `json:"category"`
	Id       int    `json:"id"`
	Name     string `json:"name"`
}

type EsiConstellation struct {
	ConstellationId int    `json:"constellation_id"`
	Name            string `json:"name"`
	RegionId        int    `json:"region_id"`
	RegionName      string `json:"region_name"`
	Systems         []int  `json:"systems"`
}

type EsiSystem struct {
	ConstellationId int     `json:"constellation_id"`
	Name            string  `json:"name"`
	SystemId        int     `json:"system_id"`
	SecurityStatus  float32 `json:"security_status"`
	SecurityClass   string  `json:"security_class"`
}

var (
	CachedNames          map[int]*EsiName = make(map[int]*EsiName)
	CachedIncursions     []*EsiIncursion
	CachedConstellations map[int]*EsiConstellation = make(map[int]*EsiConstellation)
	CachedSystems        map[int]*EsiSystem        = make(map[int]*EsiSystem)
	LastIncursionFetch   int64                     = 0
)

func GetTqStatus() *EsiStatus {
	bytes := getEndpointResult("/latest/status")
	if bytes == nil {
		return nil
	}

	var esiStatus EsiStatus
	json.Unmarshal(bytes, &esiStatus)

	return &esiStatus
}

func (server *Server) GetNames(ids *NameRequest) []*EsiName {
	// Sweet sweet copying data
	ids.Ids = UniqueInts(ids.Ids)

	bytes := postEndpointResult("/latest/universe/names", ids.Ids)

	if bytes == nil {
		return nil
	}

	var names []*EsiName
	err := json.Unmarshal(bytes, &names)

	if err != nil {
		log.Printf("Error unmarshalling json. %v", err)
		return nil
	}

	// Populate the names cache
	for _, name := range names {
		CachedNames[name.Id] = name

		bytes, _ := json.Marshal(name)
		server.Redis.Set(fmt.Sprintf("esi:names:%d", name.Id), string(bytes), 0)
	}

	return names
}

// Gets a list of the current incursions and returns whether or not it was cached or a new set
func (server *Server) GetIncursions() ([]*EsiIncursion, bool) {
	if GetEpoch()-LastIncursionFetch <= 300 {
		return CachedIncursions, false
	}

	bytes := getEndpointResult("/latest/incursions")

	if bytes == nil {
		return nil, false
	}

	var incursions []*EsiIncursion
	err := json.Unmarshal(bytes, &incursions)

	if err != nil {
		log.Printf("Error unmarshalling json. %v", err)
		return nil, false
	}

	server.PopulateIncursionData(incursions)

	CachedIncursions = incursions
	LastIncursionFetch = GetEpoch()
	return incursions, true
}

func (server *Server) PopulateIncursionData(incursions []*EsiIncursion) {
	ids := &NameRequest{
		Ids: make([]int, 0),
	}

	// Populate with names
	for _, incursion := range incursions {
		constellation := GetNameForId(incursion.ConstellationId, server.Redis)

		if constellation != nil {
			incursion.ConsellationName = constellation.Name
		} else {
			ids.Ids = append(ids.Ids, incursion.ConstellationId)
		}
	}

	if len(ids.Ids) > 0 {
		server.GetNames(ids)

		// Populate with names, again
		for _, incursion := range incursions {
			constellation := GetNameForId(incursion.ConstellationId, server.Redis)

			if constellation != nil {
				incursion.ConsellationName = constellation.Name
			}
		}
	}

	// Grab system and jump data
	for _, incursion := range incursions {
		// only fetch if we need it
		if incursion.StagingSystem == nil {
			system := server.GetSystem(incursion.StagingSolarSystemId)

			incursion.StagingSystem = system
		}

		if len(incursion.Route) <= 0 {
			route := server.GetRoute(server.Config.DefaultStagingSystemId, incursion.StagingSolarSystemId)

			incursion.Route = route
		}
	}
}

func GetNameForId(id int, redis *redis.Client) *EsiName {
	// Check memory cache first
	nameAttempt := CachedNames[id]

	// TODO: Put this check into an "isvalid" that hangs off the struct
	if nameAttempt != nil {
		return nameAttempt
	}

	// Check Redis
	// TODO: put magic string in method
	fetchCmd := redis.Get(fmt.Sprintf("esi:names:%d", id))

	// Check for existence
	if fetchCmd.Err() == nil {
		// Parse json, populate memory cache, return result
		var name EsiName
		err := json.Unmarshal([]byte(fetchCmd.Val()), &name)
		if err == nil {
			CachedNames[id] = &name
			return &name
		}
	}

	// If we fail on our two caches, return nothing, they will have to fetch.
	// The reason we don't automatically call HTTP requests here is two-fold.
	// 1. Doing an HTTP request to an external service from a seemingly innocuous method name
	// 		is really bad and will cause stalls in weird places
	// 2. Rate limiting from ESI. This method does not batch (although it could)
	return nil
}

func (server *Server) GetConstellation(id int) *EsiConstellation {
	// Check our cache first!

	var constellation EsiConstellation
	cacheAttempt := CachedConstellations[id]

	if cacheAttempt != nil {
		return cacheAttempt
	}

	cmd := server.Redis.Get(fmt.Sprintf("esi:constellations:%v", id))

	if cmd.Err() == nil {
		err := json.Unmarshal([]byte(cmd.Val()), &constellation)

		if err == nil {
			return &constellation
		}
	}

	resp := getEndpointResult(fmt.Sprintf("/latest/universe/constellations/%v", id))

	if resp == nil {
		return nil
	}

	err := json.Unmarshal(resp, &constellation)

	if err != nil {
		return nil
	}

	// Lets look up the region so it can get cached appropriately
	name := GetNameForId(constellation.RegionId, server.Redis)

	if name == nil {
		// Request name
		server.GetNames(&NameRequest{
			Ids: []int{constellation.RegionId},
		})

		name = GetNameForId(constellation.RegionId, server.Redis)
	}

	if name != nil {
		// Set it if we got it
		constellation.RegionName = name.Name
	}

	// Save to cache!
	CachedConstellations[id] = &constellation
	bytes, _ := json.Marshal(constellation)

	// It's ok if this fails
	server.Redis.Set(fmt.Sprintf("esi:constellations:%v", id), string(bytes), 0)

	return &constellation
}

func (server *Server) GetSystem(id int) *EsiSystem {

	var system EsiSystem
	cacheAttempt := CachedSystems[id]

	if cacheAttempt != nil {
		return cacheAttempt
	}

	cmd := server.Redis.Get(fmt.Sprintf("esi:systems:%v", id))

	if cmd.Err() == nil {
		err := json.Unmarshal([]byte(cmd.Val()), &system)

		if err == nil {
			return &system
		}
	}

	resp := getEndpointResult(fmt.Sprintf("/latest/universe/systems/%v", id))

	if resp == nil {
		return nil
	}

	err := json.Unmarshal(resp, &system)

	if err != nil {
		log.Printf("Error %v", err)
		return nil
	}

	// Save to cache!
	CachedSystems[id] = &system
	bytes, _ := json.Marshal(system)

	// It's ok if this fails
	server.Redis.Set(fmt.Sprintf("esi:systems:%v", id), string(bytes), 0)

	return &system
}

func (server *Server) GetRoute(src, dst int) []int {
	var jumps []int

	cmd := server.Redis.Get(fmt.Sprintf("esi:routes:%v:%v", src, dst))

	if cmd.Err() == nil {
		err := json.Unmarshal([]byte(cmd.Val()), &jumps)

		if err == nil {
			return jumps
		}
	}

	resp := getEndpointResult(fmt.Sprintf("/latest/route/%v/%v", src, dst))

	if resp == nil {
		return nil
	}

	err := json.Unmarshal(resp, &jumps)

	if err != nil {
		return nil
	}

	bytes, _ := json.Marshal(jumps)

	server.Redis.Set(fmt.Sprintf("esi:systems:%v:%v", src, dst), string(bytes), 0)

	return jumps
}

func buildUrl(path string) string {
	return fmt.Sprintf("%v%v", EsiHostname, path)
}

func getEndpointResult(path string) []byte {
	resp, err := http.DefaultClient.Get(buildUrl(path))

	if err != nil {
		log.Printf("Error requesting %v. Err: %v", path, err)
		return nil
	}

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Printf("Can't read body! %v", err)
		return nil
	}

	return body
}

func postEndpointResult(path string, v interface{}) []byte {
	content, err := json.Marshal(v)

	if err != nil {
		log.Printf("Error marshalling json")
		return nil
	}

	resp, err := http.DefaultClient.Post(buildUrl(path), "application/json", bytes.NewBuffer(content))

	if err != nil {
		log.Printf("Error making http request. %v", err)
		return nil
	}

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Printf("Can't read body. %v", err)
		return nil
	}

	return body
}
