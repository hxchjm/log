package main

import (
	"context"
	"flag"
	"github.com/hxchjm/log"
)

func main() {
	flag.Parse()
	log.Init(nil) //需要文件输出，则log.Init必不可少，否则是输出到窗口
	defer log.Close()
	ctx:=context.WithValue(context.Background(),"trace_id","1234-5678-9986-4324")
	log.SetFormat("%L %D %T  %S %F %M")
	a:=100
	log.Info(ctx,"11111 %v xxxxxx",a)
	log.Error(ctx,"2222 %v xxxxxx",a)
	log.Warn("3333 %v xxxxxx",a)

	log.Infof(ctx,"11111 %v xxxxxx",a)
	log.Errorf("2222 %v xxxxxx",a)
	log.Warnf(ctx,"3333 %v xxxxxx",a)

}

