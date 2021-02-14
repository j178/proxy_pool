package util

import (
    "fmt"
    "time"

    "github.com/koding/multiconfig"
)

var ServerConf *Config

type Config struct {
    Manager             bool   `default:"true"`       //主
    Worker              bool   `default:"true"`       //从
    RedisHost           string `default:"127.0.0.1"`  //redis host
    RedisPort           int    `default:"6379"`       //redis 端口
    RedisDb             int    `default:"1"`          //redis db
    RedisAuth           string `default:""`           //redis 密码
    PrefixKey           string `default:"proxy_pool"` //redis 默认前缀
    DataStore           string `default:"bolt"`       // 数据保存方式，bolt or redis
    DataDir             string `default:"."`          // Db 文件目录
    NewQueue            int    `default:"200"`        //验证新代理队列
    OldQueue            int    `default:"300"`        //验证旧代理队列
    Debug               bool   `default:"false"`      //调试模式
    DumpHttp            bool   `default:"false"`      //调试http
    CheckInterval       int    `default:"60"`         //检查代理间隔
    Expire              int    `default:"0"`          //redis key默认超时
    DefaultScore        int    `default:"60"`         //新代理默认分数
    Retry               int    `default:"3"`          //获取代理重试次数
    Limit               int    `default:"1000"`       //get接口默认返回limit
    Timeout             int    `default:"10"`         //爬虫默认超时
    TcpTimeout          int    `default:"4"`          //tcp池的默认超时时间
    TcpTestTimeOut      int    `default:"4"`          //tcp测试的超时时间
    ProxyTimeout        int    `default:"4"`          //测试Connect方法超时时间
    HttpsConnectTimeOut int    `default:"4"`          //反向代理时默认超时时间
    ApiBind             string `default:"0.0.0.0"`    //API的IP
    ApiPort             int    `default:"8088"`       //API的端口
    ProxyBind           string `default:"0.0.0.0"`    //动态代理的IP
    ProxyPort           int    `default:"8089"`       //动态代理的端口
    OnlyChina           bool   `default:"true"`       //只处理中国的IP
    UlimitCur           int    `default:"10240"`      //ulimit
    UlimitMax           int    `default:"10240"`      //ulimit
    ScoreAtLeast        int    `default:"60"`         //随机选择的最小分数
    MaxProxy            int    `default:"2000"`       //最大代理个数
    MaxRetry            int    `default:"3"`          //最大重试次数
    ProxyCacheTimeOut   int    `default:"60"`         //代理缓存失效时间
    EnableApi           bool   `default:"true"`       //启动API服务
    EnableProxy         bool   `default:"true"`       //启动动态代理服务
    ChromeWS            string `default:""`           //chrome's rdp ws url
}

func init() {
    var m *multiconfig.DefaultLoader
    for _, file := range []string{"config.yml", "config.yaml", "config.json", "config.toml"} {
        if FileExists(file) {
            m = multiconfig.NewWithPath(file)
            fmt.Printf("Loaded file --> %s\n", file)
            break
        }
    }
    if m == nil {
        m = multiconfig.New()
    }
    serverConf := new(Config)
    m.MustLoad(serverConf)
    ServerConf = serverConf
}

func (c Config) GetInternalCron() string {
    return fmt.Sprintf("@every %ds", c.CheckInterval)
}

func (c Config) GetTcpTestTimeOut() time.Duration {
    return time.Duration(c.TcpTestTimeOut) * time.Second
}
