package main

//如果希望日志上报到loki并在grafana中显示，移步到README.md
//go run main.go -log.agent "unixgram:///var/run/log-agent/collector.sock?timeout=100ms&chan=1024"

import (
	"context"
	"flag"
	"github.com/hxchjm/log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	flag.Parse()
	log.Init(&log.Config{
		Dir: `C:\Users\hxchj\Desktop\log`},

	) //需要文件输出，则log.Init必不可少，否则是输出到窗口
	defer log.Close()
	ctx := context.WithValue(context.Background(), "trace_id", "1234-5678-9986-4324")
	log.SetFormat("%L %D %T %i %a %S %F %M")

	a := 100
	log.Info(ctx, "11111 %v xxxxxx", a)
	log.Error(ctx, "2222 %v xxxxxx", a)
	log.Warn("3333 ", a, " xxxxxx")

	log.Infof(ctx, "11111 %v xxxxxx", a)
	log.Errorf("2222 %v xxxxxx", a)
	log.Warnf(ctx, "3333 %v xxxxxx", a)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c
		log.Info("get a signal %s", s.String())
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			log.Info("nms exit")
			time.Sleep(time.Second)
			return
		case syscall.SIGHUP:
		default:
			return
		}
	}
}
