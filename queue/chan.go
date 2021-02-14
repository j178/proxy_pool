package queue

import (
	"github.com/phpgao/proxy_pool/model"
	"github.com/phpgao/proxy_pool/util"
)

var (
	config       = util.ServerConf
	newProxyChan = make(chan *model.HttpProxy, config.NewQueue)
	oldProxyChan = make(chan *model.HttpProxy, config.OldQueue)
)

func GetNewChan() chan *model.HttpProxy {
	return newProxyChan
}

func GetOldChan() chan *model.HttpProxy {
	return oldProxyChan
}
