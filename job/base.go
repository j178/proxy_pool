package job

import (
    "errors"
    "fmt"
    "math/rand"
    "strings"
    "time"

    "github.com/antchfx/htmlquery"
    "github.com/apex/log"
    "github.com/avast/retry-go"
    "github.com/parnurzeal/gorequest"

    "github.com/phpgao/proxy_pool/db"
    "github.com/phpgao/proxy_pool/model"
    "github.com/phpgao/proxy_pool/util"
    "github.com/phpgao/proxy_pool/validator"
)

var (
    logger             = util.GetLogger("crawler")
    MaxProxyReachedErr = errors.New("max proxy reached")
    storeEngine        = db.GetDb()
    noProxy            = errors.New("no proxy")
    emptyResponse      = errors.New("empty resp")
)

func init() {
    htmlquery.DisableSelectorCache = true
}

type Crawler interface {
    Run()
    StartUrl() []string
    Cron() string
    Name() string
    Retry() uint
    NeedRetry() bool
    Enabled() bool
    // url , if use proxy
    Fetch(string, bool) (string, error)
    SetProxyChan(chan<- *model.HttpProxy)
    GetProxyChan() chan<- *model.HttpProxy
    Parse(string) ([]*model.HttpProxy, error)
}

type Spider struct {
    ch chan<- *model.HttpProxy
}

func (s *Spider) StartUrl() []string {
    panic("implement me")
}

func (s *Spider) checkErrAndStatus(errs []error, resp gorequest.Response) (err error) {
    if len(errs) > 0 {
        err = errs[0]
        return
    }
    if resp.StatusCode != 200 {
        return fmt.Errorf("http code: %d", resp.StatusCode)
    }

    return
}

func (s *Spider) Cron() string {
    panic("implement me")
}

func (s *Spider) Enabled() bool {
    return true
}

func (s *Spider) NeedRetry() bool {
    return true
}

func (s *Spider) TimeOut() int {
    return util.ServerConf.Timeout
}

func (s *Spider) Name() string {
    panic("implement me")
}

func (s *Spider) Parse(string) ([]*model.HttpProxy, error) {
    panic("implement me")
}

func (s *Spider) GetReferer() string {
    return "https://www.baidu.com/"
}

func (s *Spider) SetProxyChan(ch chan<- *model.HttpProxy) {
    s.ch = ch
}

func (s *Spider) GetProxyChan() chan<- *model.HttpProxy {
    return s.ch
}

func (s *Spider) RandomDelay() bool {
    return true
}

func (s *Spider) Retry() uint {
    return uint(util.ServerConf.MaxRetry)
}

func (s *Spider) Fetch(proxyURL string, useProxy bool) (body string, err error) {

    if s.RandomDelay() {
        time.Sleep(time.Duration(rand.Intn(6)) * time.Second)
    }

    request := gorequest.New()
    contentType := "text/html; charset=utf-8"
    var superAgent *gorequest.SuperAgent
    var resp gorequest.Response
    var errs []error
    superAgent = request.Get(proxyURL).
        Set("User-Agent", util.GetRandomUA()).
        Set("Content-Type", contentType).
        Set("Referer", s.GetReferer()).
        Set("Pragma", `no-cache`).
        Timeout(time.Duration(s.TimeOut()) * time.Second).SetDebug(util.ServerConf.DumpHttp)

    if useProxy {
        var proxy model.HttpProxy
        proxy, err = storeEngine.Random()
        if err != nil {
            return
        }
        p := "http://" + proxy.GetProxyUrl()
        logger.WithFields(log.Fields{"proxy": p, "url": proxyURL}).Debug("fetch with proxy")
        resp, body, errs = superAgent.Proxy(p).End()
    } else {
        resp, body, errs = superAgent.End()
    }
    if err = s.checkErrAndStatus(errs, resp); err != nil {
        return
    }

    body = strings.TrimSpace(body)
    return
}

func getProxy(s Crawler) {
    logger.WithField("spider", s.Name()).Debug("spider begin")
    if !s.Enabled() {
        logger.WithField("spider", s.Name()).Debug("spider is not enabled")
        return
    }
    for _, url := range s.StartUrl() {
        go func(proxySiteURL string, inputChan chan<- *model.HttpProxy) {
            defer func() {
                if r := recover(); r != nil {
                    logger.WithFields(log.Fields{
                        "url":   proxySiteURL,
                        "fatal": r,
                        "from": s.Name(),
                    }).Error("recover from error while fetching")
                }
            }()

            var newProxies []*model.HttpProxy

            var attempts = 0
            err := retry.Do(
                func() error {
                    attempts++
                    logger.WithFields(log.Fields{"attempts": attempts, "site": proxySiteURL}).Debug("fetching proxy site")

                    var err error
                    if !validator.CanDo() {
                        return MaxProxyReachedErr
                    }

                    var withProxy bool

                    if attempts > 1 {
                        withProxy = true
                    }

                    resp, err := s.Fetch(proxySiteURL, withProxy)
                    if err != nil {
                        return err
                    }

                    if resp == "" {
                        return emptyResponse
                    }

                    newProxies, err = s.Parse(resp)
                    if err != nil {
                        return err
                    }

                    if newProxies == nil {
                        return noProxy
                    }

                    return nil
                },
                retry.Attempts(s.Retry()),
                retry.RetryIf(func(err error) bool {
                    // should give up
                    if errors.Is(err, MaxProxyReachedErr) || errors.Is(err, noProxy) {
                        return false
                    }
                    return s.NeedRetry()
                }),
                retry.LastErrorOnly(true),
            )

            if err != nil {
                logger.WithError(err).WithField("url", proxySiteURL).Debug("error get new proxy")
            }

            //logger.WithFields(log.Fields{
            //	"name":  s.Name(),
            //	"url":   proxySiteURL,
            //	"count": len(newProxies),
            //}).Info("url proxy report")

            var tmpMap = map[string]int{}
            for _, newProxy := range newProxies {
                newProxy.Ip = strings.TrimSpace(newProxy.Ip)
                newProxy.Port = strings.TrimSpace(newProxy.Port)
                if _, found := tmpMap[newProxy.GetKey()]; found {
                    continue
                }
                tmpMap[newProxy.GetKey()] = 1
                newProxy.From = s.Name()
                if newProxy.Score == 0 {
                    newProxy.Score = util.ServerConf.DefaultScore
                }
                if model.FilterProxy(newProxy) {
                    inputChan <- newProxy
                }
            }
        }(url, s.GetProxyChan())
    }

}
