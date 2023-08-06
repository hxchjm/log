#使用说明
##1.本地文件输入
使用-log.dir指定日志文件的本地输出目录   

##2.loki输出并grafana显示
1. mkdir /var/run/log-agent/
2. 使用scripts/docker-compose.yaml，直接部署loki,grafana和log-agent到物理机上
3. 业务程序使用-log.agent="unixgram:///var/run/log-agent/collector.sock?timeout=100ms&chan=1024"即可
4. 为了更好地展示日志，需要在业务服务的环境变量里添加 `DEPLOY_ENV=xxx` `APP_ID=yyy`