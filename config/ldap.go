package config

import (
	"time"

	"github.com/shaj13/go-guardian/v2/auth"
	"github.com/shaj13/go-guardian/v2/auth/strategies/ldap"
	"github.com/shaj13/libcache"
)

var Strategy auth.Strategy
var CacheObj libcache.Cache

func SetupGoGuardian() {
	if !Data.LDAP.Enable {
		return
	}

	cfg := &ldap.Config{
		BaseDN:       Data.LDAP.BaseDN,
		BindDN:       Data.LDAP.BindDN,
		Port:         Data.LDAP.Port,
		Host:         Data.LDAP.Host,
		BindPassword: Data.LDAP.BindPassword,
		Filter:       Data.LDAP.Filter,
	}
	CacheObj = libcache.FIFO.New(0)
	CacheObj.SetTTL(time.Minute * 5)
	CacheObj.RegisterOnExpired(func(key, _ interface{}) {
		CacheObj.Peek(key)
	})
	Strategy = ldap.NewCached(cfg, CacheObj)

}
