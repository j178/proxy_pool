package server

import (
    "crypto/tls"
    "fmt"
    "io"
    "math/rand"
    "net"
    "net/http"
    "time"

    "github.com/apex/log"

    "github.com/phpgao/proxy_pool/cache"
    "github.com/phpgao/proxy_pool/model"
    "github.com/phpgao/proxy_pool/util"
)

var (
    timeout = time.Duration(util.ServerConf.HttpsConnectTimeOut) * time.Second
)

func handleTunneling(w http.ResponseWriter, r *http.Request) {
    var err error
    p := *cache.Cache.Get()
    proxies := p["tunnel"]
    var destConn net.Conn

    if proxies == nil {
        http.Error(w, "no proxy available", http.StatusServiceUnavailable)
        return
    } else {
        l := len(proxies)
        proxy := proxies[rand.Intn(l)]
        logger.WithField("proxy", proxy.GetProxyWithSchema()).Info("dynamic https")
        msg := fmt.Sprintf(model.ConnectCommand, http.MethodConnect, r.Host, "HTTP/1.1", r.Host)

        if proxy.IsHttps() {
            conf := &tls.Config{InsecureSkipVerify: true}
            dialer := &net.Dialer{
                Timeout: timeout,
            }
            destConn, err = tls.DialWithDialer(dialer, "tcp", proxy.GetProxyUrl(), conf)
        } else {
            destConn, err = net.DialTimeout("tcp", proxy.GetProxyUrl(), timeout)
        }
        if err != nil {
            http.Error(w, err.Error(), http.StatusServiceUnavailable)
            return
        }
        _, err = destConn.Write([]byte(msg))

        if err != nil {
            http.Error(w, err.Error(), http.StatusServiceUnavailable)
            _ = destConn.Close()
            return
        }

        err = destConn.SetReadDeadline(time.Now().Add(timeout))
        if err != nil {
            http.Error(w, err.Error(), http.StatusServiceUnavailable)
            _ = destConn.Close()
            return
        }
    }

    hijacker, ok := w.(http.Hijacker)
    if !ok {
        http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
        return
    }
    clientConn, _, err := hijacker.Hijack()
    if err != nil {
        http.Error(w, err.Error(), http.StatusServiceUnavailable)
        return
    }
    go transfer(destConn, clientConn)
    go transfer(clientConn, destConn)
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
    defer func() {
        err := destination.Close()
        if err != nil {
            logger.WithError(err).Warn("error close remote conn")
        }
        _ = source.Close()
    }()
    io.Copy(destination, source)
}

func handleHTTP(w http.ResponseWriter, req *http.Request) {
    var err error
    var Transport http.RoundTripper

    p := *cache.Cache.Get()
    proxies := p["forward"]
    if proxies == nil {
        http.Error(w, "no proxy available", http.StatusServiceUnavailable)
        return
    } else {
        proxy := proxies[rand.Intn(len(proxies))]
        logger.WithField("proxy", proxy.GetProxyWithSchema()).Debug("http forward proxy")
        Transport = &http.Transport{
            Proxy: http.ProxyURL(proxy.GetFullUrl()),
            DialContext: (&net.Dialer{
                Timeout:   timeout,
                KeepAlive: timeout,
            }).DialContext,
            ForceAttemptHTTP2:     true,
            MaxIdleConns:          100,
            IdleConnTimeout:       timeout,
            TLSHandshakeTimeout:   timeout,
            ExpectContinueTimeout: 1 * time.Second,
            // skip cert check
            TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
        }
    }

    resp, err := Transport.RoundTrip(req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusServiceUnavailable)
        return
    }
    defer resp.Body.Close()
    copyHeader(w.Header(), resp.Header)
    w.WriteHeader(resp.StatusCode)
    io.Copy(w, resp.Body)
}

func copyHeader(dst, src http.Header) {
    for k, vv := range src {
        for _, v := range vv {
            dst.Add(k, v)
        }
    }
}

// https://gist.github.com/creack/4c00ee404f2d7bd5983382cc93af5147
type middleware func(http.Handler) http.Handler
type middlewares []middleware

func (mws middlewares) apply(hdlr http.Handler) http.Handler {
    if len(mws) == 0 {
        return hdlr
    }

    return mws[1:].apply(mws[0](hdlr))
}

func logging(hdlr http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
        defer func(start time.Time) {
            logger.WithFields(log.Fields{
                "Method":     req.Method,
                "Host":       req.Host,
                "Path":       req.URL.Path,
                "RemoteAddr": req.RemoteAddr,
                "UserAgent":  req.UserAgent(),
                "Duration":   time.Since(start),
            }).Info("")
        }(time.Now())
        hdlr.ServeHTTP(w, req)
    })
}

func routerProxy() http.Handler {
    router := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodConnect {
            handleTunneling(w, r)
        } else {
            handleHTTP(w, r)
        }
    })
    return (middlewares{logging}).apply(router)
}
