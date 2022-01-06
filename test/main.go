package main

import (
	"flag"
	"gitee.com/hxchjm/log"
	"time"
)

func main(){
	flag.Parse()
	log.Init(nil) //需要文件输出，则log.Init必不可少，否则是输出到窗口
	log.SetFormat("%L %D %T  %S %F %M")
	log.Info("xxxxxxxxxxxxxxxx")
	Test()
	time.Sleep(time.Second*100)
}

func Test(){
	log.Error("xfssd ")
}