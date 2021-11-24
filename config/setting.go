package config

import (
	"fmt"

	"github.com/spf13/viper"
)

var Data Config

type AppEnv struct {
	Environment   string `mapstructure:"environment"`
	JWT           string `mapstructure:"jwtkey"`
	Port          string `mapstructure:"port"`
	AllowedOrigin string `mapstructure:"allowedorigin"`
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
	DB   Database    `mapstructure:"database"`
	App  AppEnv      `mapstructure:"app"`
	LDAP LDAPSetting `mapstructure:"ldap"`
	Seed SeedSetting `mapstructure:"seed"`
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
}
