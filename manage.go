package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jesperfj/byodemo/database"
	"github.com/jesperfj/byodemo/heroku"
	"github.com/jesperfj/byodemo/heroku/hgin"
)

// used to render orgs with accounts page
type OrgWithAccount struct {
	Organization   *heroku.Organization
	HasAccount     bool
	AWSAccessKeyId string
}

func findOrgsWithAccounts(orgs []*heroku.Organization) []*OrgWithAccount {
	result := make([]*OrgWithAccount, len(orgs))
	ids := make([]string, len(orgs))
	for i, o := range orgs {
		ids[i] = o.Id
	}
	accounts := db.FindAccounts(ids)
	for i, o := range orgs {
		key, ok := accounts[o.Id]
		result[i] = &OrgWithAccount{
			Organization:   o,
			HasAccount:     ok,
			AWSAccessKeyId: key,
		}
	}
	return result
}

func getAndValidateOrg(c *gin.Context) (org *heroku.Organization, failed bool) {
	hc := hgin.HerokuClient(c)
	orgs, err := hc.Organizations()
	if err != nil {
		c.String(500, "Oops: ", err)
		return org, true
	}
	orgId := c.Param("org_id")
	for _, o := range orgs {
		logger.Print(o.Id, " - ", o.Name)
		if o.Id == orgId {
			org = o
		}
	}
	if org == nil {
		c.String(404, "Not found.")
		return org, true
	}
	return org, false
}

func setupManageRoutes(router *gin.Engine) {
	router.GET("/callback", hgin.HandleCallback(config.cookieSecret, config.oauthSecret))

	manage := router.Group("/manage", hgin.CheckAuth(config.cookieSecret, config.oauthId))

	manage.GET("/orgs", func(c *gin.Context) {
		c.Redirect(302, "/manage/orgs/")
	})

	manage.GET("/orgs/", func(c *gin.Context) {
		hc := hgin.HerokuClient(c)
		orgs, err := hc.Organizations()
		orgsWithAccounts := findOrgsWithAccounts(orgs)
		if err != nil {
			c.String(500, "Oops: ", err)
			return
		}

		c.HTML(http.StatusOK, "orgs.tmpl.html", gin.H{"orgs": orgsWithAccounts})
	})

	manage.GET("/orgs/:org_id", func(c *gin.Context) {
		org, failed := getAndValidateOrg(c)
		if failed {
			return
		}
		c.HTML(http.StatusOK, "org.tmpl.html", gin.H{"org": org})
	})

	manage.POST("/orgs/:org_id/account", func(c *gin.Context) {
		org, failed := getAndValidateOrg(c)
		if failed {
			return
		}
		account := &database.Account{
			OwnerId:            org.Id,
			AWSAccessKeyId:     c.PostForm("awsAccessKeyId"),
			AWSSecretAccessKey: c.PostForm("awsSecretAccessKey"),
		}
		err := db.SaveAccount(account)
		if err != nil {
			logger.Print("Error saving account: ", err.Error())
			c.String(500, "Error saving account: "+err.Error())
		} else {
			c.Redirect(302, "/manage/orgs/"+c.Param("org_id"))
		}
	})

}
