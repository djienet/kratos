# Resolver

http client 的服务发现实现，默认依赖 pkg/naming 的实现，所有实现了 pkg/naming 的第三方 naming service 均可无缝接入。

## 实现原理

通过实现标准 net/http client 的 RoundTrip 接口，在 Request 中识别 URL 的 scheme，如果是注册过的 naming service 实现，则动态解析到 naming service 获取 APPID 实例地址。
