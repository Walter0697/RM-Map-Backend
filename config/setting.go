package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

var Data Config

type AppEnv struct {
	Environment                string `mapstructure:"environment"`
	JWT                        string `mapstructure:"jwtkey"`
	Port                       string `mapstructure:"port"`
	AllowedOrigin              string `mapstructure:"allowedorigin"`
	AuthMode                   string `mapstructure:"authmode"`
	AuthSessionLifetimeSeconds int    `mapstructure:"authsessionttlseconds"`
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
	Enable                      bool     `mapstructure:"enable"`
	Issuer                      string   `mapstructure:"issuer"`
	ClientID                    string   `mapstructure:"clientid"`
	ClientSecret                string   `mapstructure:"clientsecret"`
	RedirectURL                 string   `mapstructure:"redirecturl"`
	FrontendRedirectURL         string   `mapstructure:"frontendredirecturl"`
	AuthEndpoint                string   `mapstructure:"authendpoint"`
	TokenEndpoint               string   `mapstructure:"tokenendpoint"`
	UserInfoEndpoint            string   `mapstructure:"userinfoendpoint"`
	Scopes                      []string `mapstructure:"scopes"`
	UsernameClaim               string   `mapstructure:"usernameclaim"`
	DefaultRole                 string   `mapstructure:"defaultrole"`
	SessionLifetimeSeconds      int      `mapstructure:"sessionttlseconds"`
	AccessTokenLifetimeSeconds  int      `mapstructure:"accesstokenttlseconds"`
	RefreshTokenLifetimeSeconds int      `mapstructure:"refreshtokenttlseconds"`
}

type CalendarGoogleSetting struct {
	Enable              bool     `mapstructure:"enable"`
	ClientID            string   `mapstructure:"clientid"`
	ClientSecret        string   `mapstructure:"clientsecret"`
	RedirectURL         string   `mapstructure:"redirecturl"`
	FrontendRedirectURL string   `mapstructure:"frontendredirecturl"`
	AuthEndpoint        string   `mapstructure:"authendpoint"`
	TokenEndpoint       string   `mapstructure:"tokenendpoint"`
	APIBaseURL          string   `mapstructure:"apibaseurl"`
	Scopes              []string `mapstructure:"scopes"`
}

type APIKeySetting struct {
	MovieDB           string `mapstructure:"moviedb"`
	TomTomMap         string `mapstructure:"tomtommap"`
	TomTomRouting     string `mapstructure:"tomtomrouting"`
	TomTomStaticImage string `mapstructure:"tomtomstaticimage"`
	Yelp              string `mapstructure:"yelp"`
}

type ScheduleTravelSetting struct {
	Enable                    bool   `mapstructure:"enable"`
	BaseURL                   string `mapstructure:"baseurl"`
	TimeoutMS                 int    `mapstructure:"timeoutms"`
	EasyThresholdMinutes      int    `mapstructure:"easythresholdminutes"`
	DifficultThresholdMinutes int    `mapstructure:"difficultthresholdminutes"`
}

type TomTomRouteSetting struct {
	BaseURL    string `mapstructure:"baseurl"`
	TimeoutMS  int    `mapstructure:"timeoutms"`
	RetryCount int    `mapstructure:"retrycount"`
}

type TomTomStaticImageSetting struct {
	BaseURL    string `mapstructure:"baseurl"`
	TimeoutMS  int    `mapstructure:"timeoutms"`
	RetryCount int    `mapstructure:"retrycount"`
}

type WeatherSetting struct {
	Enable               bool    `mapstructure:"enable"`
	Provider             string  `mapstructure:"provider"`
	BaseURL              string  `mapstructure:"baseurl"`
	TimeoutMS            int     `mapstructure:"timeoutms"`
	RateLimitPerMinute   int     `mapstructure:"ratelimitperminute"`
	CacheTTLSeconds      int     `mapstructure:"cachettlseconds"`
	StaleTTLSeconds      int     `mapstructure:"stalettlseconds"`
	MaxForecastHours     int     `mapstructure:"maxforecasthours"`
	MaxViewportSpan      float64 `mapstructure:"maxviewportspan"`
	MaxViewportPointStep float64 `mapstructure:"maxviewportpointstep"`
}

type IntegrationAuthSetting struct {
	EnableCleanup          bool `mapstructure:"enablecleanup"`
	LogRetentionDays       int  `mapstructure:"logretentiondays"`
	MaxAuditLogRows        int  `mapstructure:"maxauditlogrows"`
	RevokedKeyRetentionDay int  `mapstructure:"revokedkeyretentiondays"`
	CleanupIntervalHours   int  `mapstructure:"cleanupintervalhours"`
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

type RedisSetting struct {
	Enable   bool   `mapstructure:"enable"`
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type AuthStateSetting struct {
	MigrationMode       string `mapstructure:"migrationmode"`
	KeyPrefix           string `mapstructure:"keyprefix"`
	SessionTTLSeconds   int    `mapstructure:"sessionttlseconds"`
	RedisDialTimeoutMS  int    `mapstructure:"redisdialtimeoutms"`
	RedisReadTimeoutMS  int    `mapstructure:"redisreadtimeoutms"`
	RedisWriteTimeoutMS int    `mapstructure:"rediswritetimeoutms"`
}

type Config struct {
	DB                Database                 `mapstructure:"database"`
	Redis             RedisSetting             `mapstructure:"redis"`
	AuthState         AuthStateSetting         `mapstructure:"authstate"`
	App               AppEnv                   `mapstructure:"app"`
	LDAP              LDAPSetting              `mapstructure:"ldap"`
	OIDC              OIDCSetting              `mapstructure:"oidc"`
	CalendarGoogle    CalendarGoogleSetting    `mapstructure:"calendargoogle"`
	APIKEY            APIKeySetting            `mapstructure:"apikey"`
	ScheduleTravel    ScheduleTravelSetting    `mapstructure:"scheduletravel"`
	TomTomRoute       TomTomRouteSetting       `mapstructure:"tomtomroute"`
	TomTomStaticImage TomTomStaticImageSetting `mapstructure:"tomtomstaticimage"`
	Weather           WeatherSetting           `mapstructure:"weather"`
	IntegrationAuth   IntegrationAuthSetting   `mapstructure:"integrationauth"`
	Seed              SeedSetting              `mapstructure:"seed"`
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

	if err := applyRedisEnvOverrides(); err != nil {
		panic(fmt.Errorf("invalid redis config: %s \n", err))
	}

	if err := ValidateAuthConfig(); err != nil {
		panic(fmt.Errorf("invalid auth config: %s \n", err))
	}

	if mode, err := ResolveAuthMode(); err == nil {
		log.Printf("Authentication mode resolved to: %s", mode)
	}
}

func applyRedisEnvOverrides() error {
	if dbValue, exists := os.LookupEnv("REDIS_DB"); exists {
		parsedDB, err := strconv.Atoi(strings.TrimSpace(dbValue))
		if err != nil {
			return fmt.Errorf("REDIS_DB must be an integer, got %q", dbValue)
		}
		if parsedDB < 0 {
			return fmt.Errorf("REDIS_DB must be >= 0, got %d", parsedDB)
		}
		Data.Redis.DB = parsedDB
	}
	if modeValue, exists := os.LookupEnv("AUTH_STATE_MIGRATION_MODE"); exists {
		Data.AuthState.MigrationMode = strings.TrimSpace(modeValue)
	}
	if ttlValue, exists := os.LookupEnv("AUTH_STATE_SESSION_TTL_SECONDS"); exists {
		parsedTTL, err := strconv.Atoi(strings.TrimSpace(ttlValue))
		if err != nil {
			return fmt.Errorf("AUTH_STATE_SESSION_TTL_SECONDS must be an integer, got %q", ttlValue)
		}
		Data.AuthState.SessionTTLSeconds = parsedTTL
	}
	if lifetimeValue, exists := os.LookupEnv("AUTH_SESSION_LIFETIME_SECONDS"); exists {
		parsedLifetime, err := strconv.Atoi(strings.TrimSpace(lifetimeValue))
		if err != nil {
			return fmt.Errorf("AUTH_SESSION_LIFETIME_SECONDS must be an integer, got %q", lifetimeValue)
		}
		if parsedLifetime <= 0 {
			return fmt.Errorf("AUTH_SESSION_LIFETIME_SECONDS must be > 0, got %d", parsedLifetime)
		}
		Data.App.AuthSessionLifetimeSeconds = parsedLifetime
	}
	if easyThresholdValue, exists := os.LookupEnv("SCHEDULE_TRAVEL_EASY_THRESHOLD_MINUTES"); exists {
		parsedEasyThreshold, err := strconv.Atoi(strings.TrimSpace(easyThresholdValue))
		if err != nil {
			return fmt.Errorf("SCHEDULE_TRAVEL_EASY_THRESHOLD_MINUTES must be an integer, got %q", easyThresholdValue)
		}
		Data.ScheduleTravel.EasyThresholdMinutes = parsedEasyThreshold
	}
	if difficultThresholdValue, exists := os.LookupEnv("SCHEDULE_TRAVEL_DIFFICULT_THRESHOLD_MINUTES"); exists {
		parsedDifficultThreshold, err := strconv.Atoi(strings.TrimSpace(difficultThresholdValue))
		if err != nil {
			return fmt.Errorf("SCHEDULE_TRAVEL_DIFFICULT_THRESHOLD_MINUTES must be an integer, got %q", difficultThresholdValue)
		}
		Data.ScheduleTravel.DifficultThresholdMinutes = parsedDifficultThreshold
	}
	if enabledValue, exists := os.LookupEnv("SCHEDULE_TRAVEL_ENABLE"); exists {
		switch strings.ToLower(strings.TrimSpace(enabledValue)) {
		case "1", "true", "yes", "on":
			Data.ScheduleTravel.Enable = true
		case "0", "false", "no", "off":
			Data.ScheduleTravel.Enable = false
		default:
			return fmt.Errorf("SCHEDULE_TRAVEL_ENABLE must be a boolean, got %q", enabledValue)
		}
	}
	if timeoutValue, exists := os.LookupEnv("SCHEDULE_TRAVEL_TIMEOUT_MS"); exists {
		parsedTimeout, err := strconv.Atoi(strings.TrimSpace(timeoutValue))
		if err != nil {
			return fmt.Errorf("SCHEDULE_TRAVEL_TIMEOUT_MS must be an integer, got %q", timeoutValue)
		}
		Data.ScheduleTravel.TimeoutMS = parsedTimeout
	}
	if baseURLValue, exists := os.LookupEnv("SCHEDULE_TRAVEL_BASE_URL"); exists {
		Data.ScheduleTravel.BaseURL = strings.TrimSpace(baseURLValue)
	}
	return nil
}
