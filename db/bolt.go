package db

import (
    "encoding/json"
    "path/filepath"
    "time"

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

func (self *boltDB) Init() error {
    if err := util.EnsureDir(self.DataDir); err != nil {
        return err
    }

    db, err := bolt.Open(filepath.Join(self.DataDir, "proxies.db"), 0600, nil)
    if err != nil {
        return err
    }

    self.db = db
    self.BucketName = []byte(util.ServerConf.PrefixKey)

    err = db.Update(func(tx *bolt.Tx) error {
        _, err := tx.CreateBucketIfNotExists(self.BucketName)
        return err
    })
    if err != nil {
        return err
    }
    return nil
}

func (self *boltDB) Close() error {
    return self.db.Close()
}

func (self *boltDB) KeyExist(key string) bool {
    exist := false
    err := self.db.View(func(tx *bolt.Tx) error {
        bucket := tx.Bucket(self.BucketName)
        value := bucket.Get([]byte(key))
        exist = value != nil
        return nil
    })
    if err != nil {
        return false
    }
    return exist
}

func (self *boltDB) put(bucket *bolt.Bucket, key string, proxy model.HttpProxy, deadline *time.Time) error {
    dict := proxy.GetProxyMap()
    if deadline != nil {
        deadlineText, err := deadline.MarshalText()
        if err != nil {
            return err
        }
        dict["Deadline"] = deadlineText
    }

    data, err := json.Marshal(dict)
    if err != nil {
        return err
    }
    err = bucket.Put([]byte(key), data)
    return err
}

func (self *boltDB) get(bucket *bolt.Bucket, key string) (proxy model.HttpProxy, err error) {
    valueBytes := bucket.Get([]byte(key))
    if valueBytes == nil {
        return
    }
    var m map[string]string
    err = json.Unmarshal(valueBytes, &m)
    if err != nil {
        return
    }
    if _, ok := m["Deadline"]; ok {
        t := new(time.Time)
        err = t.UnmarshalText([]byte(m["Deadline"]))
        if err != nil {
            return
        }
        delete(m, "Deadline")
        if t.Before(time.Now()) {
            return
        }
    }

    return model.Make(m)
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
    if !self.KeyExist(key) {
        err := self.db.Update(func(tx *bolt.Tx) error {
            bucket := tx.Bucket(self.BucketName)
            return self.put(bucket, key, proxy, self.getDeadline())
        })
        return err == nil
    } else {
        err := self.AddScore(proxy, 10)
        if err != nil {
            return false
        }
    }
    return true
}

func (self *boltDB) GetAll() []model.HttpProxy {
    var deviceTokenBytes []byte
    err := self.db.View(func(tx *bolt.Tx) error {
        bucket := tx.Bucket([]byte("device"))
        deviceTokenBytes = bucket.Get([]byte(key))
        if deviceTokenBytes == nil {
            return errors.New("没找到 DeviceToken")
        }
        return nil
    })
    if err != nil {
        return "", err
    }

    return string(deviceTokenBytes), nil
}

func (self *boltDB) Get(m map[string]string) ([]model.HttpProxy, error) {
    err := self.db.View(func(tx *bolt.Tx) error {
        bucket := tx.Bucket(self.BucketName)

    })
}

func (self *boltDB) Exists(proxy model.HttpProxy) bool {
    key := proxy.GetKey()
    return self.KeyExist(key)
}

func (self *boltDB) Remove(proxy model.HttpProxy) error {
    key := proxy.GetKey()
    return self.db.Update(func(tx *bolt.Tx) error {
        bucket := tx.Bucket(self.BucketName)
        return bucket.Delete([]byte(key))
    })
}

func (self *boltDB) RemoveAll(proxies []model.HttpProxy) error {
    err := self.db.Update(func(tx *bolt.Tx) error {
        return tx.DeleteBucket(self.BucketName)
    })
    if err != nil {
        return err
    }
    return self.db.Update(func(tx *bolt.Tx) error {
        _, err := tx.CreateBucketIfNotExists(self.BucketName)
        return err
    })
}

func (self *boltDB) Random() (model.HttpProxy, error) {
    panic("implement me")
}

func (self *boltDB) Len() int {
    var keyN int
    _ = self.db.View(func(tx *bolt.Tx) error {
        bucket := tx.Bucket(self.BucketName)
        stats := bucket.Stats()
        keyN = stats.KeyN
        return nil
    })
    return keyN
}

func (self *boltDB) Test() bool {
    return true
}

func (self *boltDB) UpdateSchema(proxy model.HttpProxy) error {
    panic("implement me")
}

func (self *boltDB) AddScore(key model.HttpProxy, score int) error {
    panic("implement me")
}
