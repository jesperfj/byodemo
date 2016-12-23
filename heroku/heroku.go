package heroku

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

var (
	logger = log.New(os.Stderr, "[heroku] ", log.Ldate|log.Ltime|log.Lshortfile)
)

type Client struct {
	Authorization *Authorization
}

type Authorization struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type AddonApp struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type Addon struct {
	App AddonApp `json:"app"`
}

type App struct {
	Organization AppOrganization `json:"organization"`
	Owner        AppOwner        `json:"owner"`
}

type AppOrganization struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type AppOwner struct {
	Id    string `json:"id"`
	Email string `json:"email"`
}

type AddonConfig struct {
	Config []ConfigVar `json:"config"`
}

type ConfigVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type AddonPlanChangeRequest struct {
	Plan  string `json:"plan"`
	AppId string `json:"heroku_id"`
	Uuid  string `json:"uuid"`
}

type AddonPlanChangeResponse struct {
	Message string            `json:"message"`
	Config  map[string]string `json:"config"`
}

type AddonOAuthGrant struct {
	Code      string `json:"code"`
	ExpiresAt string `json:"expires_at"`
	GrantType string `json:"type"`
}

type CreateAddonRequest struct {
	CallbackUrl  string            `json:"callback_url"`
	HerokuId     string            `json:"heroku_id"`
	Plan         string            `json:"plan"`
	Region       string            `json:"region"`
	OAuthGrant   AddonOAuthGrant   `json:"oauth_grant"`
	LogplexToken string            `json:"logplex_token"`
	Uuid         string            `json:"uuid"`
	Options      map[string]string `json:"options"`
}

type CreateAddonResponse struct {
	Id     string            `json:"id"`
	Config map[string]string `json:"config"`
}

type AsyncCreateAddonResponse struct {
	Id      string `json:"id"`
	Message string `json:"message"`
}

type Organization struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Role    string `json:"role"`
	Type    string `json:"type"`
	Default bool   `json:"default"`
}

type Account struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func NewAddonId() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func NewClientFromCode(clientSecret string, code string) (*Client, error) {
	logger.Print(clientSecret)
	res, err := http.PostForm("https://id.heroku.com/oauth/token",
		url.Values{
			"grant_type":    {"authorization_code"},
			"client_secret": {clientSecret},
			"code":          {code},
		},
	)
	if err != nil {
		logger.Print(err)
		return nil, err
	}
	defer res.Body.Close()

	if err := httpError(200, res); err != nil {
		logger.Print(err.Error())
		return nil, err
	}

	authInfo := &Authorization{}
	err = json.NewDecoder(res.Body).Decode(authInfo)
	if err != nil {
		return nil, err
	}

	return &Client{Authorization: authInfo}, nil
}

func (c *Client) addHeaders(req *http.Request) {
	req.Header.Add("Accept", "application/vnd.heroku+json; version=3")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", c.Authorization.TokenType+" "+c.Authorization.AccessToken)
}

// Time to collapse some verbosity. Using this now just for getting orgs
func (c *Client) get(path string, responseData interface{}) error {
	req, err := http.NewRequest("GET", "https://api.heroku.com"+path, nil)
	if err != nil {
		return err
	}
	c.addHeaders(req)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	err = json.NewDecoder(res.Body).Decode(responseData)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) AddonInfo(addonId string) (*Addon, error) {
	req, err := http.NewRequest("GET", "https://api.heroku.com/addons/"+addonId, nil)
	if err != nil {
		return nil, err
	}
	c.addHeaders(req)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	addonInfo := &Addon{}
	err = json.NewDecoder(res.Body).Decode(addonInfo)
	if err != nil {
		return nil, err
	}
	return addonInfo, nil
}

func (c *Client) OwnerId(addonId string) (ownerId string, err error) {
	addonInfo, err := c.AddonInfo(addonId)
	req, err := http.NewRequest("GET", "https://api.heroku.com/apps/"+addonInfo.App.Id, nil)
	logError(err)
	c.addHeaders(req)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ownerId, err
	}
	appInfo := &App{}
	err = json.NewDecoder(res.Body).Decode(appInfo)
	logError(err)
	ownerId = appInfo.Owner.Id
	return ownerId, nil
}

func (c *Client) ProvisionAddon(addonId string, success bool) (err error) {
	endpoint := "provision"
	if !success {
		endpoint = "deprovision"
	}
	req, err := http.NewRequest("POST", "https://api.heroku.com/addons/"+addonId+"/actions/"+endpoint, nil)
	logError(err)
	c.addHeaders(req)
	res, err := http.DefaultClient.Do(req)
	defer res.Body.Close()
	if err != nil {
		return err
	}
	if err := httpError(200, res); err != nil {
		logger.Print(err.Error())
		return err
	}
	return nil
}

func (c *Client) CompleteProvisioning(addonId string) (err error) {
	return c.ProvisionAddon(addonId, true)
}

func (c *Client) FailProvisioning(addonId string) (err error) {
	return c.ProvisionAddon(addonId, false)
}

// Example:
// c.SetAddonConfig("1234", heroku.AddonConfig{
//    Config: []heroku.ConfigVar{
//      heroku.ConfigVar{
//        Name:  "URL",
//        Value: "https://blahblah.com",
//      },
//      heroku.ConfigVar{
//        Name:  "OTHER",
//        Value: "value",
//      },
//    },
//  })
func (c *Client) SetAddonConfig(addonId string, config AddonConfig) error {
	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(config)
	logError(err)

	req, err := http.NewRequest("PATCH", "https://api.heroku.com/addons/"+addonId+"/config", b)
	logError(err)
	c.addHeaders(req)

	res, err := http.DefaultClient.Do(req)
	defer res.Body.Close()
	if err != nil {
		return err
	}
	if err := httpError(200, res); err != nil {
		logger.Print(err.Error())
		return err
	}
	return nil
}

func (c *Client) Organizations() ([]*Organization, error) {
	orgs := make([]*Organization, 0)
	err := c.get("/organizations", &orgs)
	return orgs, err
}

func (c *Client) Account() (account *Account, err error) {
	err = c.get("/account", &account)
	return account, err
}

func logError(err error) {
	if err != nil {
		logger.Print("WARNING! Ignoring unexpected error: %s", err.Error())
	}
}

func httpError(expectedCode int, res *http.Response) error {
	if expectedCode != res.StatusCode {
		body, _ := httputil.DumpResponse(res, true)
		return errors.New(fmt.Sprintf("Unexpected HTTP response (%d): %q", res.StatusCode, body))
	} else {
		return nil
	}
}
