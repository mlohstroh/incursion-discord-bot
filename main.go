package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/joho/godotenv"
)

type Server struct {
	Redis   *redis.Client
	Discord *discordgo.Session
	Config  *Config
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	port, err := strconv.Atoi(os.Getenv("PORT"))

	if err != nil {
		port = 3000
	}
	config := ParseConfig()
	server := NewServer(config)

	discordWait := make(chan bool)

	server.Config = config
	go server.SetupDiscord(config, discordWait)

	// Wait for discord to say that they're connected
	<-discordWait

	server.RegisterCommands()

	go server.RunScheduler()

	router := gin.Default()
	SetupRoutes(router)

	router.Run(fmt.Sprintf(":%d", port))
}

func SetupRoutes(router *gin.Engine) {
	router.GET("/", rootPath)
	router.GET("/discord", AddBot)
	router.GET("/discord/auth", discordAuth)
}

func rootPath(c *gin.Context) {
	c.String(http.StatusOK, "Stayin' alive...")
}

func discordAuth(c *gin.Context) {
	c.String(http.StatusOK, "Added to discord. Enjoy...")
}

func AddBot(c *gin.Context) {
	// TODO: change this
	c.Redirect(http.StatusTemporaryRedirect, os.Getenv("DISCORD_ADD_BOT_OAUTH"))
}

func NewServer(config *Config) *Server {
	redis := NewRedis()

	return &Server{
		Redis:  redis,
		Config: config,
	}
}

// TODO: Put in own file
func (server *Server) RunScheduler() {
	scheduler := NewScheduler(time.Minute)

	scheduler.Schedule("IncursionChecker", server.checkIncursions, time.Minute*5+time.Second)

	if len(os.Getenv("HOSTED_URL")) > 0 {
		scheduler.Schedule("HerokuKeepAlive", server.herokuKeepAlive, time.Minute*20)
	}

	scheduler.Run()

	if err := server.SetupIncursions(); err != nil {
		// Pop off this just to check in case of no incursions saved
		server.checkIncursions()
	}
}

var (
	lastIncursions []*EsiIncursion
	deadIncursions []*EsiIncursion
)

func (server *Server) herokuKeepAlive() {
	// From: https://devcenter.heroku.com/articles/free-dyno-hours#dyno-sleeping
	// # Dyno sleeping
	// If an app has a web dyno, and that web dyno receives no traffic in a 30 minute period, the web dyno will sleep. In addition to the web dyno sleeping, the worker dyno (if present) will also sleep.

	// Free dynos do not consume Free dyno hours while sleeping.

	// If a sleeping web dyno receives web traffic, it will become active again after a short delay. If the app has a worker dyno that was scaled up before sleeping, it will be scaled up again too.

	url := os.Getenv("HOSTED_URL")

	_, err := http.DefaultClient.Get(url)

	if err != nil {
		panic(err)
	}
}
