package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis"
	"log"
	"os"
	"strings"
)

var (
	Guilds map[string]*discordgo.Guild = make(map[string]*discordgo.Guild, 0)
)

func (server *Server) SetupDiscord(config *Config, waitChan chan bool) {
	discord, err := discordgo.New("Bot " + os.Getenv("DISCORD_BOT_TOKEN"))

	if err != nil {
		panic(fmt.Sprintf("Unable to set up Discord! Check the bot token! Error: %v", err))
	}

	if err = discord.Open(); err != nil {
		panic(fmt.Sprintf("Error opening Discord Session. %v", err))
	}
	defer discord.Close()

	discord.AddHandler(server.OnMessageCreate)
	discord.AddHandler(server.OnGuildJoin)

	log.Printf("Connected to Discord...")

	// Block on this function call

	// Send that we have been setup
	waitChan <- true

	// TODO: this feels bad, probably want to return it on the channel
	server.Discord = discord

	// Block on this channel
	<-waitChan
	log.Printf("Disconnected to Discord...")
}

func (server *Server) OnMessageCreate(s *discordgo.Session, message *discordgo.MessageCreate) {
	// Ignore if it came from us, always
	if message.Author.ID == s.State.User.ID {
		return
	}

	if Exists(server.Config.DiscordIncursionChannels, message.ChannelID) {
		if strings.HasPrefix(message.Content, "!") {
			split := strings.Split(message.Content, " ")
			commandCenter.ProcessCommand(split[0], s, message)
		}
	}
}

// Note: This gets called on startup
func (server *Server) OnGuildJoin(s *discordgo.Session, event *discordgo.GuildCreate) {
	log.Printf("Joined guild %v", event.Guild.Name)
	Guilds[event.Guild.ID] = event.Guild
}

func (server *Server) OnGuildLeave(s *discordgo.Session, event *discordgo.GuildDelete) {
	log.Printf("Left guild %v", event.Guild.Name)
	delete(Guilds, event.Guild.ID)
}

func (server *Server) BroadcastMessage(message string) {
	for id, _ := range Guilds {
		channel, err := GetBroadcastChannelForGuild(server.Redis, id)

		if err != nil {
			log.Printf("Error on broadcast. Guild has not set up broadcast channel!")
			continue
		}

		server.SendMessage(channel, message)
	}
}

func (server *Server) SendMessage(channel, message string) {
	_, err := server.Discord.ChannelMessageSend(channel, message)
	if err != nil {
		log.Printf("Error sending message %v", err)
		return
	}
}

func (server *Server) SendDirectMessage(user *discordgo.User, message string) {
	channel, err := server.Discord.UserChannelCreate(user.ID)

	if err != nil {
		log.Printf("Error creating PM for user %v", user.ID)
		return
	}

	server.SendMessage(channel.ID, message)
}

func GetBroadcastChannelForGuild(redis *redis.Client, guildId string) (string, error) {
	channel := redis.Get(fmt.Sprintf("discord:%v:broadcast_channel", guildId))
	return channel.Val(), channel.Err()
}

func SetBroadcastChannelForGuild(redis *redis.Client, guildId, channelId string) {
	redis.Set(fmt.Sprintf("discord:%v:broadcast_channel", guildId), channelId, 0)
}
