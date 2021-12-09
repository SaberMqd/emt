#emt

emt 基础服务框架
emt-grpcgateway http转grpc的短连接网关
emt-agent 长连接网关
emt-login 长连接登录服
emt-cell 长连接基础后端服务
emt-cellmgr 长连接基础后端管理服务

emt

  server
    grpc
    tcp
    udp
    ws
    kcp
    http

  client
    grpc
    tcp
    udp
    kcp
    ws
    http

  subscriber
    rabbit
    redis

  publisher
    rabbit
    redis

  registry
    zookeeper
    consul
    mdns

  codec
    json
    protobuf

  log
    zap

  config
    nacos
    json
    yml

  selector
    random

  sync

  cache

emt-grpcgateway

emt-agent

emt-login

emt-cell

emt-cellmgr

emt-agentmgr
