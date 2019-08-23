package main

import (
	"bytes"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"strings"
)

type DiscordCommand = func(*discordgo.Session, *discordgo.MessageCreate)

type CommandCenter struct {
	Commands map[string]DiscordCommand
}

// TODO: I dislike this global variable
var commandCenter CommandCenter

func (server *Server) RegisterCommands() {
	commandCenter.Commands = make(map[string]DiscordCommand)
	commandCenter.Commands["!incursions"] = server.HandleIncursion
	commandCenter.Commands["!status"] = server.HandleTqStatus
	commandCenter.Commands["!testping"] = server.TestPing
	commandCenter.Commands["!instructions"] = server.GetInstructions
	commandCenter.Commands["!setinstructions"] = server.SetInstructions
	commandCenter.Commands["!setadmin"] = server.SetAdmin
	commandCenter.Commands["!removeadmin"] = server.RemoveAdmin
	commandCenter.Commands["!setbroadcast"] = server.SetBroadcastChannel
	commandCenter.Commands["!broadcast"] = server.TestBroadcast
}

func (commandCenter *CommandCenter) ProcessCommand(command string, session *discordgo.Session, message *discordgo.MessageCreate) {
	if commandCenter.Commands[command] != nil {
		commandCenter.Commands[command](session, message)
	}
}

func (server *Server) HandleIncursion(session *discordgo.Session, message *discordgo.MessageCreate) {
	incursions, _ := server.GetIncursions()

	buffer := bytes.NewBufferString("")
	for _, inc := range incursions {
		server.GetDefaultIncurionsMessage(inc, buffer)
	}

	if buffer.Len() <= 0 {
		buffer.WriteString("No Null or Low Sec Incursions... Go Krab!")
	}

	_, err := session.ChannelMessageSend(message.ChannelID, buffer.String())
	
	if err != nil {
		log.Print(err)
	}
}

func (server *Server) HandleTqStatus(session *discordgo.Session, message *discordgo.MessageCreate) {
	log.Printf("Retrieving Tranquility Status")

	tq := GetTqStatus()

	if tq == nil {
		session.ChannelMessageSend(message.ChannelID, "Tranquility is offline.")
		return
	}

	session.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Tranquility is online with %v players.", tq.Players))
}

func (server *Server) TestPing(session *discordgo.Session, message *discordgo.MessageCreate) {
	messages := "this was a mistake.\n"
	buffer := bytes.NewBufferString(messages)

	for _, mention := range server.Config.DiscordAdmins {
		buffer.WriteString(fmt.Sprintf("<@%v> ", mention))
	}

	for _, channel := range server.Config.DiscordIncursionChannels {
		server.SendMessage(channel, buffer.String())
	}
}

func (server *Server) GetInstructions(session *discordgo.Session, message *discordgo.MessageCreate) {
	cmd := server.Redis.Get("bot:instructions")
	if cmd.Err() != nil {
		log.Printf("Unable to get instructions. Error: %v", cmd.Err())
		server.SendDirectMessage(message.Author, "No instructions are set")
		return
	}

	server.SendDirectMessage(message.Author, cmd.Val())
}

func (server *Server) SetInstructions(session *discordgo.Session, message *discordgo.MessageCreate) {

	if !Exists(server.Config.DiscordAdmins, message.Author.ID) {
		// Invalid admin
		server.SendMessage(message.ChannelID, "Please don't try to set instructions if you aren't authorized")
		return
	}

	instructions := strings.Replace(message.Content, "!setinstructions", "", -1)

	cmd := server.Redis.Set("bot:instructions", instructions, 0)

	if cmd.Err() != nil {
		log.Printf("Error setting instructions %v", cmd.Err())
		server.SendMessage(message.ChannelID, fmt.Sprintf("Unable to set instructions. Error: %v", cmd.Err()))
		return
	}

	server.SendMessage(message.ChannelID, fmt.Sprintf(`Instructions set to "%v"`, instructions))
}

func (server *Server) SetAdmin(session *discordgo.Session, message *discordgo.MessageCreate) {
	adminId := strings.TrimSpace(strings.Replace(message.Content, "!setadmin", "", -1))
	server.Redis.SAdd("incursions:admins", adminId)

	server.SendMessage(message.ChannelID, fmt.Sprintf("<@%v> added as admin", adminId))
}

func (server *Server) RemoveAdmin(session *discordgo.Session, message *discordgo.MessageCreate) {
	adminId := strings.TrimSpace(strings.Replace(message.Content, "!removeadmin", "", -1))

	server.Redis.SRem("incursions:admins", adminId)

	server.SendMessage(message.ChannelID, fmt.Sprintf("<@%v> removed as admin", adminId))
}

func (server *Server) SetBroadcastChannel(session *discordgo.Session, message *discordgo.MessageCreate) {
	channelId := strings.Replace(message.Content, "!setbroadcast", "", -1)
	channelId = strings.TrimSpace(channelId)

	wasSet := false

	// Have to look through all channels we can see
	for id, guild := range Guilds {
		for _, channel := range guild.Channels {
			if channel.ID == channelId {
				wasSet = true
				SetBroadcastChannelForGuild(server.Redis, id, channelId)
			}
		}
	}

	if wasSet {
		server.SendMessage(message.ChannelID, fmt.Sprintf("Broadcast channel was set to %v", channelId))
	} else {
		server.SendMessage(message.ChannelID, "Could not find channel ID in any of my registered servers. Please try again")
	}
}

func (server *Server) TestBroadcast(session *discordgo.Session, message *discordgo.MessageCreate) {
	msg := strings.Replace(message.Content, "!broadcast", "", -1)

	server.BroadcastMessage(msg)
}
	msg := strings.Replace(message.Content, "!broadcast", "", -1)

	server.BroadcastMessage(msg)
}
