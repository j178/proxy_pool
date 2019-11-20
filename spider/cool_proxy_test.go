package spider

import (
	"fmt"
	"github.com/phpgao/proxy_pool/model"
	"testing"
	"time"
)

func TestCoolProxy_Fetch(t *testing.T) {
	newProxyChan := make(chan *model.HttpProxy, 100)
	spider := CoolProxy{Spider{ch: newProxyChan}}
	spider.Run()
	timeout := time.After(30 * time.Second)
	for {
		select {
		case proxy := <-newProxyChan:
			fmt.Println(proxy)
		case <-timeout:
			fmt.Println("There's no more time to this. Exiting!")
			return
		}
	}
}
