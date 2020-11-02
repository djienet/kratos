![kratos](doc/img/kratos3.png)

# Kratos

Kratos 是 B 站开源的一套 Go 微服务框架，包含大量微服务相关框架及工具。  

> 名字来源于:《战神》游戏以希腊神话为背景，讲述由凡人成为战神的奎托斯（Kratos）成为战神并展开弑神屠杀的冒险历程。

## Goals

我们致力于提供完整的微服务研发体验，整合相关框架及工具后，微服务治理相关部分可对整体业务开发周期无感，从而更加聚焦于业务交付。对每位开发者而言，整套Kratos 框架也是不错的学习仓库。

## Features
* HTTP Blademaster：核心基于[gin](https://github.com/gin-gonic/gin)进行模块化设计，简单易用、核心足够轻量；
* GRPC Warden：基于官方gRPC开发，集成[discovery](https://github.com/bilibili/discovery)服务发现，并融合P2C负载均衡；
* Cache：优雅的接口化设计，非常方便的缓存序列化，推荐结合代理模式[overlord](https://github.com/bilibili/overlord)；
* Database：集成MySQL/HBase/TiDB，添加熔断保护和统计支持，可快速发现数据层压力；
* Config：方便易用的[paladin sdk](doc/wiki-cn/config.md)，可配合远程配置中心，实现配置版本管理和更新；
* Log：类似[zap](https://github.com/uber-go/zap)的field实现高性能日志库，并结合log-agent实现远程日志管理；
* Trace：基于opentracing，集成了全链路trace支持（gRPC/HTTP/MySQL/Redis/Memcached）；
* Kratos Tool：工具链，可快速生成标准项目，或者通过Protobuf生成代码，非常便捷使用gRPC、HTTP、swagger文档；

## Quick start

### Requirments

1. Golang 版本要求：go version>=1.13
2. 设置环境变量：
   ```shell
      # 强制使用 go module 模式
      export GO111MODULE=on

      # 将 GOPATH 加入环境路径
      export PATH=$PATH:$GOPATH/bin

      # 设置启用 goproxy 代理，加速国外包
      # https://goproxy.cn 是七牛维护的国内镜像代理，若有问题，还可使用官方代理：https://goproxy.io
      go env -w GOPROXY=https://goproxy.cn,direct 

      # 我们使用的是 coding 私有仓库，需设置以域名"github.com"开头的仓库为私有包，不走代理，不校验
      # 若未来迁移至其它私有仓库，也需要设置
      go env -w GOPRIVATE=github.com
   ```
3. 安装 protobuffer 编译器 `protoc`（注意：高版本的 protobuf 会导致引用的 grpc 版本升高，导致编译失败）
   ```
      Linux:
      wget https://github.com/protocolbuffers/protobuf/releases/download/v3.7.1/protoc-3.7.1-linux-x86_64.zip
      unzip protoc-3.7.1-linux-x86_64.zip
      cd protoc-3.7.1 && mv bin/protoc $GOPATH/bin/ && mv include/ $GOPATH/

   ```

### Installation

1. 联系管理员申请 github.com/djienet/kratos 私有仓库权限。
2. 根据访问私有 coding 仓库的方式，设置 git：
   * 方式一：配置通过私有证书访问 coding 仓库（使用 `git clone git@github.com:azoya/xxx` 方式）。但当我们执行 `go get github.com/djienet/kratos` 安装时，go 默认会使用 `git clone https://github.com/azoya/xxx` 的方式访问目标仓库，因此，需设置 git clone https 私用仓库时强制使用 SSH（git@github.com）协议。

      ``` shell
         git config --global url.git@github.com:.insteadOf https://github.com/
      ```

   * 方式二：未配置证书访问 coding 仓库，每次克隆仓库时需手动输入用户名/密码（`git clone https://github.com/azoya/xxx` 方式）。需要设置 git 记住用户名密码，因为 `go get github.com/djienet/kratos` 时会屏蔽输入，你无法手动输入用户名密码。

      ``` shell
         git config --global credential.helper store
      ```

      **注意**: 初次安装，请手动 `git clone https://github.com/djienet/kratos` 且输入用户名/密码，让 git 记住用户密码。
3. 安装框架 kratos 工具：

   ``` shell
      go get -u github.com/djienet/kratos/tool/kratos
   ```

   检查：

   ``` shell
      kratos -v
   ```
   
   能输出版本信息，则安装成功。

### Generate Demo & Build & Run

```shell
kratos new kratos-demo
cd kratos-demo/cmd
go build
./cmd -conf ../configs
```

打开浏览器访问：[http://localhost:8000/kratos-demo/start](http://localhost:8000/kratos-demo/start)，你会看到输出了`Golang 大法好 ！！！`

## Documentation

> [简体中文](doc/wiki-cn/summary.md)  
> [FAQ](doc/wiki-cn/FAQ.md)  

## License
Kratos is under the MIT license. See the [LICENSE](./LICENSE) file for details.

-------------

*Please report bugs, concerns, suggestions by issues, or join QQ-group 716486124 to discuss problems around source code.*
