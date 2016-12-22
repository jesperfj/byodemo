package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jesperfj/byodemo/database"
)

type appConfig struct {
	clientSecret       string
	port               string
	dbCreds            string
	dbSecret           string
	addonProviderToken string
	cookieSecret       string
	oauthId            string
	oauthSecret        string
}

var (
	logger = log.New(os.Stderr, "[Web] ", log.Ldate|log.Ltime|log.Lshortfile)
	db     database.DbController
	config appConfig
)

func check(e error) bool {
	if e != nil {
		fmt.Println("Error: " + e.Error())
		return false
	}
	return true
}

func getRequiredenv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		logger.Fatal(key, " must be set")
	}
	return val
}

func main() {
	config = appConfig{
		port:               getRequiredenv("PORT"),
		dbCreds:            getRequiredenv("DATABASE_URL"),
		dbSecret:           getRequiredenv("DATABASE_SECRET"),
		addonProviderToken: getRequiredenv("ADDON_PROVIDER_TOKEN"),
		cookieSecret:       getRequiredenv("COOKIE_SECRET"),
		oauthId:            getRequiredenv("HEROKU_OAUTH_ID"),
		oauthSecret:        getRequiredenv("HEROKU_OAUTH_SECRET"),
		clientSecret:       getRequiredenv("ADDON_PROVIDER_CLIENT_SECRET"),
	}

	// Need to declare err in advance, because cannot use := syntax in next statement as that
	// will create a main() scoped db variable when we really want a globally scoped variable.
	var err error
	db, err = database.NewController(config.dbCreds, config.dbSecret)
	if err != nil {
		logger.Fatal("Error connecting to database: ", err)
	}

	// General routing setup
	router := gin.New()
	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*.tmpl.html")
	router.Static("/static", "static")

	// Root redirect
	router.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/manage/orgs/")
	})

	// Backplane likes to hit this endpoint, so let's respond.
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "")
	})

	// Utility endpoint to generate a new database key
	// It was easier to put it here than create a new cmd
	router.GET("/genkey", func(c *gin.Context) {
		c.String(http.StatusOK, database.GenerateEncodedKey())
	})

	// Test endpoint for setting up AWS accounts without the UI
	router.POST("/account", func(c *gin.Context) {
		account := &database.Account{}
		c.Bind(account)
		logger.Print("Saving new AWS creds for Heroku user ", account.OwnerId)
		err := db.SaveAccount(account)
		if err != nil {
			logger.Print("Error saving account: ", err.Error())
			c.String(500, "Error saving account: "+err.Error())
		} else {
			c.String(201, "Account saved.")
		}
	})

	// Management Endpoints

	setupManageRoutes(router)

	// Heroku Addon Endpoints

	setupAddonRoutes(router)

	router.Run(":" + config.port)
}
