package db

import (
	"fmt"
	"github.com/go-redis/redis/v7"
	"github.com/phpgao/proxy_pool/model"
	"github.com/phpgao/proxy_pool/util"
)

var (
	config = util.ServerConf
	logger = util.GetLogger("db")
	db     Store
)

type Store interface {
	Init() error
	GetAll() []model.HttpProxy
	Get(map[string]string) ([]model.HttpProxy, error)
	Exists(model.HttpProxy) bool
	Add(model.HttpProxy) bool
	UpdateSchema(model.HttpProxy) error
	Remove(model.HttpProxy) error
	RemoveAll([]model.HttpProxy) error
	Random() (model.HttpProxy, error)
	Len() int
	Test() bool
	AddScore(key model.HttpProxy, score int) error
}

func GetDb() Store {
	if db == nil {
		db = &redisDB{
			client: redis.NewClient(&redis.Options{
                    Addr:     fmt.Sprintf("%s:%d", config.RedisHost, config.RedisPort),
                    Password: config.RedisAuth, // no password set
                    DB:       config.RedisDb,   // use default DB
			}),
			PrefixKey: config.PrefixKey,
			KeyExpire: config.Expire,
		}
		if !db.Test() {
			panic("db test error")
		}
	}

	return db
}
