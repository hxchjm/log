#使用说明
##1.本地文件输入
使用-log.dir指定日志文件的本地输出目录   

##2.loki输出并grafana显示
1. mkdir /var/run/log-agent/
2. 使用scripts/docker-compose.yaml，直接部署loki,grafana和log-agent到物理机上
3. 业务程序使用-log.agent="unixgram:///var/run/log-agent/collector.sock?timeout=100ms&chan=1024"即可
4. 为了更好地展示日志，需要在业务服务的环境变量里添加 `DEPLOY_ENV=xxx` `APP_ID=yyy`


## 3.逻辑
(f *FileWriter) Write ->  (f *FileWriter) daemon()
1. daemon中的select分为两部分，第一部分实时写入bufio中，纯内存操作，加快速度。

文件写入逻辑：
1. 调用(f *FileWriter) Write方法，其中会获取池化的bytes.Buffer对象填充数据，然后发送到管道里。
2. 循环携程(f *FileWriter) daemon()会监听管道，收到数据，并调用f.writer.Write
3. 上述f.writer对象赋值在filewriter.New()->initFileRotate->Reset()中完成。