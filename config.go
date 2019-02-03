package main

import (
	"encoding/json"
	"io/ioutil"
)

type Config struct {
	DiscordIncursionChannels []string `json:"discord_incursion_channels"`
	DiscordAdmins            []string `json:"discord_admins"`
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

func (server *Server) PostConfigLoad() {
	admins := server.Redis.SMembers("incursions:admins")
	if admins.Err() == nil {
		slice := admins.Val()

		for _, admin := range slice {
			server.Config.DiscordAdmins = append(server.Config.DiscordAdmins, admin)
		}
	}
}
