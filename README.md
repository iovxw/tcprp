# tcprp
TCP Reverse Proxy，可反向代理任何内网TCP链接

## 编译

```
go get github.com/Bluek404/tcprp/...
```

## 使用

客户端放在需要代理的内网服务器上

`./client 目标服务器 代理服务器 密钥`

例如: `./client 127.0.0.1:8080 example.com:8091 KEYKEY` (此处example.com代指服务器地址)

服务端放在有公网IP的服务器上

`./server 服务端口 代理通信端口 密钥`

例如: `./server :80 :8091 KEYKEY`

然后用户访问*example.com:80*就可以了
