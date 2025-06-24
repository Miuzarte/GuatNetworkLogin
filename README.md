# GuatNetworkLogin

## 👻✈️校园网自动登录

Usage: GuatNetworkLogin \<account> \<password> \<ISP>

ISP: 0:校园网, 1:电信, 2:联通, 3:移动

每天 `6:30` 尝试登录校园网(持续五分钟),
不检测具体返回的内容,
GET成功即算作登录成功

或者按回车直接执行一次

设计为在路由器上运行, 内存占用应该也许大概不会超过 10MiB

不放心的还可以设置环境变量 `GOMEMLIMIT=8MiB` 限制 go runtime 的内存使用

`Windows x64` 平台下, 运行后程序待机时 `占用1376K` `提交12908K`
