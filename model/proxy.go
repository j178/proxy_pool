package model

import (
    "crypto/md5"
    "crypto/tls"
    "encoding/hex"
    "errors"
    "fmt"
    "io"
    "io/ioutil"
    "net"
    "net/http"
    "net/url"
    "reflect"
    "strconv"
    "strings"
    "time"

    "github.com/fatih/structs"

    "github.com/phpgao/proxy_pool/util"
)

var (
    config         = util.ServerConf
    tcpTestTimeout = config.GetTcpTestTimeOut()
    logger         = util.GetLogger("model")
    proxyNotWork   = errors.New("proxy not work")
)

const (
    ConnectCommand = "%s %s %s\r\nHost: %s\r\nProxy-Connection: Keep-Alive\r\n\r\n"
    testUrl        = "http://ip.cip.cc"
    testHttpsUrl   = "https://ip.cip.cc"
)

type HttpProxy struct {
    Ip        string `json:"ip"`
    Port      string `json:"port"`
    Schema    string `json:"schema"`
    Tunnel    bool   `json:"tunnel"`
    Score     int    `json:"score"`
    Latency   int    `json:"latency"`
    From      string `json:"from"`
    Anonymous int    `json:"anonymous"`
    Country   string `json:"country"`
    Deadline  string `json:"deadline"`
}

func Make(m map[string]string) (newProxy HttpProxy, err error) {
    rVal := reflect.ValueOf(&newProxy).Elem()
    rType := reflect.TypeOf(newProxy)
    fieldCount := rType.NumField()

    for i := 0; i < fieldCount; i++ {
        t := rType.Field(i)
        f := rVal.Field(i)
        if v, ok := m[t.Name]; ok {
            ddd := reflect.TypeOf(v)
            if ddd != t.Type {
                v, _ := strconv.Atoi(v)
                f.Set(reflect.ValueOf(v))
            } else {
                f.Set(reflect.ValueOf(v))
            }
        } else {
            return newProxy, errors.New(t.Name + " not found")
        }
    }

    defer func() {
        if r := recover(); r != nil {
            logger.WithField("fatal", r).Warn("Recovered")
        }
    }()

    return
}

func (p *HttpProxy) GetKey() string {
    hash := md5.New()
    _, err := io.WriteString(hash, p.GetProxyUrl())
    if err != nil {
        return ""
    }
    return hex.EncodeToString(hash.Sum(nil))
}

func (p *HttpProxy) GetProxyUrl() string {
    return fmt.Sprintf("%s:%s", p.Ip, p.Port)
}

func (p *HttpProxy) GetProxyWithSchema() string {
    // 默认所有的 proxy 都是 http 协议
    return fmt.Sprintf("%s://%s:%s", p.Schema, p.Ip, p.Port)
}

func (p *HttpProxy) GetFullUrl() *url.URL {
    _url := p.GetProxyWithSchema()
    u, err := url.Parse(_url)
    if err != nil {
        panic("invalid proxy url" + _url)
    }
    return u
}

func (p *HttpProxy) GetProxyMap() map[string]interface{} {
    return structs.Map(p)
}

func (p *HttpProxy) GetIp() string {
    return p.Ip
}

func (p *HttpProxy) GetPort() string {
    return p.Port
}
func (p *HttpProxy) IsHttps() bool {
    return p.Schema == "https"
}

func (p *HttpProxy) GetHttpTransport() *http.Transport {
    t := &http.Transport{
        Proxy:           http.ProxyURL(p.GetFullUrl()),
        TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
    }
    return t
}

// test tcp
func (p *HttpProxy) TestTcp() error {
    conn, err := net.DialTimeout("tcp", p.GetProxyUrl(), tcpTestTimeout)
    if conn != nil {
        _ = conn.Close()
    }
    return err
}

func (p *HttpProxy) TestTls() error {
    conf := &tls.Config{
        InsecureSkipVerify: true,
    }
    dialer := &net.Dialer{
        Timeout: tcpTestTimeout,
    }
    conn, err := tls.DialWithDialer(dialer, "tcp", p.GetProxyUrl(), conf)
    if conn != nil {
        _ = conn.Close()
    }
    return err
}

func (p *HttpProxy) testProxy(target string) (err error) {
    timeout := 6 * time.Second

    client := &http.Client{
        Transport: p.GetHttpTransport(),
        Timeout:   timeout,
    }

    resp, err := client.Get(target)
    if err != nil {
        return
    }
    defer func() {
        err_ := resp.Body.Close()
        if err != nil {
            return
        }
        err = err_
        return
    }()

    if resp.StatusCode != 200 {
        return fmt.Errorf("http code %d", resp.StatusCode)
    }

    b, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return
    }
    html := strings.TrimSpace(string(b))
    if html != p.GetIp() {
        return proxyNotWork
    }
    return
}

func (p *HttpProxy) TestProxy() (err error) {
    return p.testProxy(testUrl)
}

// test http connect method
func (p *HttpProxy) TestHttpTunnel() (err error) {
    // target scheme == https 时， net/http 会使用 CONNECT 方式建立隧道
    // 所以可以通过这种方式来检测 proxy 是否支持 http tunnel
    return p.testProxy(testHttpsUrl)
}
