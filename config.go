package main

import (
	"encoding/json"
	"io/ioutil"
	"fmt"
)

type Config struct {
	DefaultStagingSystemId   int      `json:"default_staging_system_id"`
	SecurityStatusThreshold  float32  `json:"security_status_threshold"`
}

func ParseConfig() *Config {
	contents, err := ioutil.ReadFile("config.json")

	if err != nil {
		panic("Unable to read config file!")
	}

	var config Config
	err = json.Unmarshal(contents, &config)

	if err != nil {
		panic("Malformed json in config.json!")
	}

	return &config
}

// GetAdminsForGuild is just a redis call, so this is super fast
func (server *Server) GetAdminsForGuild(guildId string) []string {
	admins := server.Redis.SMembers(fmt.Sprintf("incursions:%v:admins", guildId))

	if admins.Err() == nil {
		// TODO: We should do validation here
		return admins.Val()
	}

	return make([]string, 0)
}