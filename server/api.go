package server

import (
    "math/rand"
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"

    "github.com/phpgao/proxy_pool/db"
    "github.com/phpgao/proxy_pool/job"
    "github.com/phpgao/proxy_pool/model"
    "github.com/phpgao/proxy_pool/util"
)

var (
    logger      = util.GetLogger("api")
    storeEngine = db.GetDb()
)

const home = "https://github.com/phpgao/proxy_pool"

type Resp struct {
    Code   int         `json:"code"`
    Error  string      `json:"error"`
    Total  int         `json:"total"`
    Score  interface{} `json:"score,omitempty"`
    Tunnel int         `json:"tunnel"`
    Cn     int         `json:"cn,omitempty"`
    Data   interface{} `json:"data"`
    Get    string      `json:"get,omitempty"`
    Random string      `json:"random,omitempty"`
    Home   string      `json:"home,omitempty"`
}

func routerApi() http.Handler {
    if !util.ServerConf.Debug {
        gin.SetMode(gin.ReleaseMode)
    }
    e := gin.Default()
    e.GET("/", handlerStatus)
    e.GET("/get", handlerQuery)
    e.GET("/random", handlerRandom)
    e.GET("/random_text", handlerRandomText)

    return e
}

func handlerQuery(c *gin.Context) {
    var err error
    resp := Resp{
        Code: http.StatusOK,
    }

    proxies, err := Filter(c)

    if err != nil {
        resp.Error = err.Error()
    } else {
        resp.Data = proxies
        resp.Total = len(proxies)
    }

    c.JSON(http.StatusOK, resp)
}

func handlerStatus(c *gin.Context) {
    resp := Resp{
        Code: http.StatusOK,
    }
    allSpiders := job.ListOfSpider
    status := make(map[string]int)
    for _, s := range allSpiders {
        status[s.Name()] = 0
    }
    scores := make(map[string]int)
    tunnels := 0
    cn := 0

    proxies := storeEngine.GetAll()
    l := len(proxies)
    if l > 0 {
        resp.Total = len(proxies)
        for _, p := range proxies {
            if p.Tunnel {
                tunnels++
            }
            if p.Country == "cn" {
                cn++
            }
            status[p.From] ++
            setDefault(scores, strconv.Itoa(p.Score), 0, 1)
        }
    }
    resp.Data = status
    resp.Score = scores
    resp.Tunnel = tunnels
    resp.Cn = cn
    resp.Home = home
    resp.Get = "/get?schema=&score="
    resp.Random = "/random?schema=&score="
    c.JSON(http.StatusOK, resp)
}

func handlerRandom(c *gin.Context) {
    resp := Resp{
        Code: http.StatusOK,
    }
    proxies, err := Filter(c)
    if err != nil {
        resp.Error = err.Error()
        c.JSON(http.StatusOK, resp)
        return
    }

    if len(proxies) == 0 {
        resp.Error = "no proxy"
        c.JSON(http.StatusOK, resp)
        return
    }
    resp.Data = proxies[rand.Intn(len(proxies))]
    resp.Total = len(proxies)

    c.JSON(http.StatusOK, resp)
}

func handlerRandomText(c *gin.Context) {
    proxies, err := Filter(c)
    if err != nil {
        c.String(http.StatusOK, "")
        return
    }

    if len(proxies) == 0 {
        c.String(http.StatusOK, "")
        return
    }

    c.String(http.StatusOK, proxies[rand.Intn(len(proxies))].GetProxyWithSchema())
}

func Filter(c *gin.Context) (proxies []model.HttpProxy, err error) {
    tunnel := c.Query("tunnel")
    // score above given number
    score := c.Query("score")
    latency := c.Query("latency")
    _source := c.Query("source")
    country := c.Query("country")
    limit := c.DefaultQuery("limit", "0")

    return storeEngine.Get(map[string]string{
        "tunnel":  tunnel,
        "score":   score,
        "source":  _source,
        "country": country,
        "limit":   limit,
        "latency": latency,
    })
}

func setDefault(h map[string]int, k string, v, inc int) (set bool, r int) {
    if _, set = h[k]; !set {
        h[k] = v
        set = true
    }
    h[k] += inc
    return
}
