package validator

import (
    "sync"
    "time"

    "github.com/phpgao/proxy_pool/db"
    "github.com/phpgao/proxy_pool/model"
    "github.com/phpgao/proxy_pool/queue"
    "github.com/phpgao/proxy_pool/util"
)

var (
    config      = util.ServerConf
    storeEngine = db.GetDb()
    lockMap     = sync.Map{}
)

func NewValidator() {
    q := queue.GetNewChan()
    var wg sync.WaitGroup
    for i := 0; i < config.NewQueue; i++ {
        wg.Add(1)
        go func() {
            for {
                proxy := <-q

                func(p *model.HttpProxy) {
                    var err error
                    logger := util.GetLogger("validator_new").WithField("from", p.From)
                    key := p.GetKey()
                    if _, ok := lockMap.Load(key); ok {
                        return
                    }

                    lockMap.Store(key, 1)
                    defer lockMap.Delete(key)

                    if storeEngine.Exists(*p) {
                        logger.WithField("proxy", proxy.GetProxyUrl()).Infof("proxy existed, ignore it")
                        return
                    }

                    err = p.TestTcp()
                    if err != nil {
                        logger.WithError(err).WithField("proxy", p.GetProxyUrl()).Debug("test tcp error")
                        return
                    }
                    err = p.TestTls()
                    if err != nil {
                        logger.WithError(err).WithField("proxy", p.GetProxyUrl()).Debug("test tls error")
                        p.Schema = "http"
                    } else {
                        p.Schema = "https"
                    }
                    startsAt := time.Now()
                    err = p.TestProxy()
                    if err != nil {
                        logger.WithError(err).WithField("proxy", p.GetProxyUrl()).Debug("test http proxy error")
                        return
                    } else {
                        p.Latency = int(time.Since(startsAt) / time.Millisecond)
                        err = p.TestHttpTunnel()
                        if err != nil {
                            logger.WithError(err).WithField("proxy", p.GetProxyUrl()).Debug("test http tunnel proxy error")
                        } else {
                            p.Tunnel = true
                        }
                    }
                    logger.WithField("proxy", p.GetProxyUrl()).Info("added new proxy")
                    storeEngine.Add(*p)
                }(proxy)
            }
        }()
    }
    wg.Wait()
}
