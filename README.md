# rcomproxy
A simple proxy


## Run
```
#proxy server
go run *.go
```

## Build
```
docker build -t rcomproxy .
```

## Run in Docker
```
#start one proxy server
docker run -it --name proxy --rm -p 3128:3128 \
rcomproxy
```

## Push to Docker Hub
```
v=latest
docker tag rcomproxy:latest zuyong/rcomproxy:${v}
docker push zuyong/rcomproxy:${v}

```

## Run on remote server
```
hostip=10.90.7.56

ssh root@${hostip} docker pull zuyong/rcomproxy:${v}

ssh root@${hostip}
docker run -it --name proxy --rm -p 4128:3128 zuyong/rcomproxy

```
