A Telegram bot for media download, using [github.com/gotd/td](https://github.com/gotd/td).
## Build
```
go build -o telegram-td-bot main.go object.go
```

## Usage
edit serve.sh or serve.cmd and just run it.
<br>
params:
```
apiId 123456

apiHash xxx

botToken xxx:xxx_xxx-x

saveDir <path>

proxyIp <socks5 proxy ip>

proxyPort <port>

proxyAuth <user>

proxyPwd <password>

onMessage <true|false> default true

onChannelMessage <true|false> default true

threadNum <max concurrent download number> default 3
```
or use command line to run, see serve.sh/serve.cmd
