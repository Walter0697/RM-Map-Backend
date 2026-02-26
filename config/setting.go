package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

var Data Config

type AppEnv struct {
	Environment   string `mapstructure:"environment"`
	JWT           string `mapstructure:"jwtkey"`
	Port          string `mapstructure:"port"`
	AllowedOrigin string `mapstructure:"allowedorigin"`
	AuthMode      string `mapstructure:"authmode"`
}

type Database struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Name     string `mapstructure:"dbname"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type LDAPSetting struct {
	Enable       bool   `mapstructure:"enable"`
	BaseDN       string `mapstructure:"basedn"`
	BindDN       string `mapstructure:"binddn"`
	Port         string `mapstructure:"port"`
	Host         string `mapstructure:"host"`
	BindPassword string `mapstructure:"bindpassword"`
	Filter       string `mapstructure:"filter"`
	DefaultRole  string `mapstructure:"defaultrole"`
}

type OIDCSetting struct {
	Enable              bool     `mapstructure:"enable"`
	Issuer              string   `mapstructure:"issuer"`
	ClientID            string   `mapstructure:"clientid"`
	ClientSecret        string   `mapstructure:"clientsecret"`
	RedirectURL         string   `mapstructure:"redirecturl"`
	FrontendRedirectURL string   `mapstructure:"frontendredirecturl"`
	AuthEndpoint        string   `mapstructure:"authendpoint"`
	TokenEndpoint       string   `mapstructure:"tokenendpoint"`
	UserInfoEndpoint    string   `mapstructure:"userinfoendpoint"`
	Scopes              []string `mapstructure:"scopes"`
	UsernameClaim       string   `mapstructure:"usernameclaim"`
	DefaultRole         string   `mapstructure:"defaultrole"`
}

type APIKeySetting struct {
	MovieDB   string `mapstructure:"moviedb"`
	TomTomMap string `mapstructure:"tomtommap"`
}
type SeedSetting struct {
	EnableSeed       bool    `mapstructure:"enableSeed"`
	MarkerNums       int     `mapstructure:"markerNums"`
	ScheduleDays     int     `mapstructure:"scheduleDays"`
	MarkerRelationId uint    `mapstructure:"markerRelationId"`
	CreateUserId     uint    `mapstructure:"createUserId"`
	CenterLatitude   float64 `mapstructure:"centerLatitude"`
	CenterLongitude  float64 `mapstructure:"centerLongitude"`
	CenterOffset     float64 `mapstructure:"centerOffset"`
}

type Config struct {
	DB     Database      `mapstructure:"database"`
	App    AppEnv        `mapstructure:"app"`
	LDAP   LDAPSetting   `mapstructure:"ldap"`
	OIDC   OIDCSetting   `mapstructure:"oidc"`
	APIKEY APIKeySetting `mapstructure:"apikey"`
	Seed   SeedSetting   `mapstructure:"seed"`
}

func Init() {
	viper.SetConfigFile("config.toml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	if err := viper.Unmarshal(&Data); err != nil {
		panic(fmt.Errorf("unable to decode into struct: %s \n", err))
	}

	if err := ValidateAuthConfig(); err != nil {
		panic(fmt.Errorf("invalid auth config: %s \n", err))
	}

	if mode, err := ResolveAuthMode(); err == nil {
		log.Printf("Authentication mode resolved to: %s", mode)
	}
}
