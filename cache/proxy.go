package cache

import (
	"strconv"
	"sync"
	"time"

	"github.com/phpgao/proxy_pool/db"
	"github.com/phpgao/proxy_pool/model"
	"github.com/phpgao/proxy_pool/util"
)

type Cached struct {
	m       sync.RWMutex
	proxies map[string][]model.HttpProxy
	expire  time.Time
	expired bool
}

var (
	logger = util.GetLogger("cache")
	Cache  = Cached{
		proxies: map[string][]model.HttpProxy{},
	}
	engine       = db.GetDb()
	cacheTimeout = time.Duration(util.ServerConf.ProxyCacheTimeOut)
)

func init() {
	Cache.proxies = getProxyMap()
	Cache.expire = time.Now().Add(cacheTimeout * time.Second)
}

func getProxyMap() map[string][]model.HttpProxy {
	m := map[string][]model.HttpProxy{
		"forward":  nil,
		"tunnel": nil,
	}
	var err error
	for k := range m {
		var p []model.HttpProxy

		if k == "forward" {
			p, err = engine.Get(map[string]string{
				"score": strconv.Itoa(util.ServerConf.ScoreAtLeast),
			})
		} else {
			p, err = engine.Get(map[string]string{
				"score":  strconv.Itoa(util.ServerConf.ScoreAtLeast),
				"tunnel": "true",
			})
		}

		if err != nil {
			logger.WithField("type", k).WithError(err).Error("get proxy error")
			continue
		}
		m[k] = p
	}

	return m
}

func (c *Cached) Get() *map[string][]model.HttpProxy {
	c.m.RLock()
	defer c.m.RUnlock()
	if time.Now().After(c.expire){
		c.Update()
	}
	return &Cache.proxies
}

func (c *Cached) Update() {
	logger.Info("updating cache")
	c.proxies = getProxyMap()
	c.expire = time.Now().Add(cacheTimeout * time.Second)
	logger.Info("update cache done")
}
