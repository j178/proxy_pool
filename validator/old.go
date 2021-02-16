package validator

import (
    "sync"

    "github.com/apex/log"

    "github.com/phpgao/proxy_pool/model"
    "github.com/phpgao/proxy_pool/queue"
    "github.com/phpgao/proxy_pool/util"
)

func OldValidator() {
    q := queue.GetOldChan()
    var wg sync.WaitGroup
    logger := util.GetLogger("validator_old")

    for i := 0; i < config.OldQueue; i++ {
        wg.Add(1)
        go func() {
            for {
                proxy := <-q
                func(p model.HttpProxy) {
                    key := p.GetKey()
                    if _, ok := lockMap.Load(key); ok {
                        return
                    }

                    lockMap.Store(key, 1)
                    defer func() {
                        lockMap.Delete(key)
                    }()

                    if !storeEngine.Exists(p) {
                        return
                    }

                    var score int
                    err := p.TestTcp()
                    if err != nil {
                        logger.WithError(err).WithField("proxy", p.GetProxyWithSchema()).Debug("test tcp error")
                        score = -100
                    } else {
                        score = 10
                        err := p.TestProxy()
                        if err != nil {
                            logger.WithError(err).WithField("proxy", p.GetProxyWithSchema()).Debug("test http tunnel error")
                            score = -100
                        }
                    }
                    logger.WithFields(log.Fields{
                        "score": score,
                        "proxy": p.GetProxyWithSchema(),
                    }).Info("set score")

                    err = storeEngine.AddScore(p, score)
                    if err != nil {
                        logger.WithError(err).WithField("proxy", p.GetProxyWithSchema()).Error("set score error")
                    }

                }(*proxy)
            }

        }()

    }
    wg.Wait()
}
