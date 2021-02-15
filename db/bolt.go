package db

import (
    "encoding/json"
    "math/rand"
    "path/filepath"
    "strconv"
    "time"

    "github.com/pkg/errors"
    bolt "go.etcd.io/bbolt"

    "github.com/phpgao/proxy_pool/model"
    "github.com/phpgao/proxy_pool/util"
)

type boltDB struct {
    db         *bolt.DB
    BucketName []byte
    KeyExpire  int
    DataDir    string
}

var (
    keyExpired   = errors.New("key expired")
    keyNotExists = errors.New("key not exists")
    noProxy      = errors.New("no proxy")
    invalidJson  = errors.New("invalid json")
)

func (self *boltDB) Init() error {
    if err := util.EnsureDir(self.DataDir); err != nil {
        logger.WithError(err).Error("ensure dir error")
        return err
    }

    options := *bolt.DefaultOptions
    options.Timeout = time.Second
    db, err := bolt.Open(
        filepath.Join(self.DataDir, "proxies.db"),
        0600,
        &options,
    )
    if err != nil {
        logger.WithError(err).Error("open db error")
        return err
    }

    self.db = db
    self.BucketName = []byte(config.PrefixKey)

    err = db.Update(func(tx *bolt.Tx) error {
        _, err := tx.CreateBucketIfNotExists(self.BucketName)
        return err
    })
    if err != nil {
        logger.WithError(err).Error("create bucket error")
        return err
    }
    return nil
}

func (self *boltDB) Close() error {
    err := self.db.Close()
    if err != nil {
        logger.WithError(err).Error("close db error")
    }
    return err
}

func (self *boltDB) GetByKey(key string) (model.HttpProxy, error) {
    var proxy model.HttpProxy

    err := self.db.View(func(tx *bolt.Tx) error {
        bucket := tx.Bucket(self.BucketName)
        value := bucket.Get([]byte(key))
        if value == nil {
            return keyNotExists
        }
        var err error
        proxy, err = self.unmarshal(value)
        if err != nil {
            return err
        }
        return nil
    })
    return proxy, err
}

func (self *boltDB) marshal(proxy model.HttpProxy, deadline *time.Time) ([]byte, error) {
    dict := proxy.GetProxyMap()
    if deadline != nil {
        deadlineText, err := deadline.MarshalText()
        if err != nil {
            return nil, err
        }
        dict["Deadline"] = deadlineText
    }

    data, err := json.Marshal(dict)
    if err != nil {
        return nil, err
    }
    return data, err
}

func (self *boltDB) unmarshal(data []byte) (model.HttpProxy, error) {
    if data == nil {
        return model.HttpProxy{}, invalidJson
    }
    var p model.HttpProxy
    err := json.Unmarshal(data, &p)
    if err != nil {
        return model.HttpProxy{}, err
    }
    if expired(&p) {
        return model.HttpProxy{}, keyExpired
    }

    return p, nil
}

func expired(p *model.HttpProxy) bool {
    deadline := p.Deadline
    if deadline != "" {
        t := new(time.Time)
        err := t.UnmarshalText([]byte(deadline))
        if err != nil {
            return true
        }
        if t.Before(time.Now()) {
            return true
        }
    }
    return false
}

func (self *boltDB) getDeadline() *time.Time {
    if self.KeyExpire <= 0 {
        return nil
    }
    deadline := time.Now().Add(time.Second * time.Duration(self.KeyExpire))
    return &deadline
}

func (self *boltDB) Add(proxy model.HttpProxy) bool {
    key := proxy.GetKey()
    _, err := self.GetByKey(key)
    if err == nil {
        err := self.AddScore(proxy, 10)
        if err != nil {
            logger.WithError(err).Error("add score error")
            return false
        }
    } else if err == keyNotExists {
        err := self.db.Update(func(tx *bolt.Tx) error {
            bucket := tx.Bucket(self.BucketName)
            data, err := self.marshal(proxy, self.getDeadline())
            if err == nil {
                err = bucket.Put([]byte(key), data)
            }
            return err
        })
        if err != nil {
            logger.WithError(err).Error("add proxy error")
            return false
        }
    } else {
        logger.WithError(err).Error("get by key error")
        return false
    }
    return true
}

func (self *boltDB) GetAll() []model.HttpProxy {
    var proxies []model.HttpProxy

    err := self.db.View(func(tx *bolt.Tx) error {
        bucket := tx.Bucket(self.BucketName)
        cursor := bucket.Cursor()
        for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
            proxy, err := self.unmarshal(v)
            if err != nil {
                return err
            }
            proxies = append(proxies, proxy)
        }
        return nil
    })
    if err != nil {
        logger.WithError(err).Error("get all error")
        return nil
    }
    return proxies
}

func (self *boltDB) Get(options map[string]string) (proxies []model.HttpProxy, err error) {
    all := self.GetAll()
    filters, err := model.GetNewFilter(options)
    if err != nil {
        return
    }
    var limit int
    if l, ok := options["limit"]; ok {
        limit, err = strconv.Atoi(l)
        if err != nil {
            return
        }
    }
    if limit == 0 {
        limit = config.Limit
    }
    if len(filters) > 0 {
        for _, p := range all {
            if Match(filters, p) {
                proxies = append(proxies, p)
            }
            if len(proxies) > limit {
                return
            }
        }
    } else {
        if len(all) <= limit {
            proxies = all
        } else {
            proxies = all[:limit]
        }
    }
    return
}

func (self *boltDB) Exists(proxy model.HttpProxy) bool {
    key := proxy.GetKey()
    _, err := self.GetByKey(key)
    return err == nil
}

func (self *boltDB) Remove(proxy model.HttpProxy) error {
    key := proxy.GetKey()
    err := self.db.Update(func(tx *bolt.Tx) error {
        bucket := tx.Bucket(self.BucketName)
        return bucket.Delete([]byte(key))
    })
    if err != nil {
        logger.WithError(err).Error("remove error")
    }
    return err
}

func (self *boltDB) RemoveAll(proxies []model.HttpProxy) error {
    proxyKeys := make(map[string]bool, len(proxies))
    for _, p := range proxies {
        proxyKeys[p.GetKey()] = true
    }

    err := self.db.Batch(func(tx *bolt.Tx) error {
        bucket := tx.Bucket(self.BucketName)
        cursor := bucket.Cursor()
        for k, _ := cursor.First(); k != nil; k, _ = cursor.Next() {
            if _, ok := proxyKeys[string(k)]; ok {
                err := cursor.Delete()
                if err != nil {
                    return err
                }
            }
        }
        return nil
    })
    if err != nil {
        logger.WithError(err).Error("remove all error")
    }
    return err
}

func (self *boltDB) Random() (model.HttpProxy, error) {
    var proxy model.HttpProxy
    keyCount := self.Len()
    if keyCount <= 0 {
        return model.HttpProxy{}, noProxy
    }
    keyIndex := rand.Intn(keyCount)
    err := self.db.View(func(tx *bolt.Tx) error {
        bucket := tx.Bucket(self.BucketName)
        cursor := bucket.Cursor()
        i := 0
        var err error
        for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
            if i == keyIndex {
                proxy, err = self.unmarshal(v)
                return err
            }
            i += 1
        }
        return noProxy
    })
    if err != nil {
        logger.WithError(err).Error("get random error")
    }
    return proxy, err
}

func (self *boltDB) Len() int {
    var keyN int
    err := self.db.View(func(tx *bolt.Tx) error {
        bucket := tx.Bucket(self.BucketName)
        stats := bucket.Stats()
        keyN = stats.KeyN
        return nil
    })
    if err != nil {
        logger.WithError(err).Error("get len error")
    }
    return keyN
}

func (self *boltDB) Test() bool {
    return true
}

func (self *boltDB) UpdateSchema(proxy model.HttpProxy) error {
    key := proxy.GetKey()
    data, err := self.GetByKey(key)
    if err != nil {
        return keyNotExists
    }
    err = self.db.Update(func(tx *bolt.Tx) error {
        bucket := tx.Bucket(self.BucketName)
        data.Schema = proxy.Schema
        value, err := self.marshal(data, self.getDeadline())
        if err != nil {
            return err
        }
        return bucket.Put([]byte(key), value)
    })
    if err != nil {
        logger.WithError(err).Error("update schema error")
    }
    return err
}

func (self *boltDB) AddScore(proxy model.HttpProxy, score int) error {
    key := proxy.GetKey()
    data, err := self.GetByKey(key)
    if err != nil {
        return keyNotExists
    }
    rs := data.Score + score
    if rs < 0 {
        return self.Remove(proxy)
    }
    if rs >= 100 {
        rs = 100
    }
    proxy.Score = rs
    data.Score = rs

    err = self.db.Update(func(tx *bolt.Tx) error {
        bucket := tx.Bucket(self.BucketName)
        value, err := self.marshal(data, self.getDeadline())
        if err != nil {
            return err
        }
        return bucket.Put([]byte(key), value)
    })

    if err != nil {
        logger.WithError(err).Error("update score error")
    }
    return err
}
