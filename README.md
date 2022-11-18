# elsa

###  概述


elsa 是一个基于 Google 的 RPC 框架协议( gRPC )开发服务框架,填补了开发者在使用 gRPC 时遇到服务注册、服务发现、链路追踪、服务监控、服务熔断降级等一些列微服务架构中服务治理问题的空白。


### elsa 特性

1. 服务注册与发现
2. 负载均衡
3. 链路追踪
4. 服务监控
5. 服务限流熔断降级


### 快速开始


##### 1. *PingService.proto*


``` protobuf 
syntax ="proto3";

package  io.busgo.ping;
option  go_package ="./pb";


service PingService {
    rpc Ping(PingRequest)returns(PingResponse);
}



message  PingRequest {
  string  ping =1;
}

message  PingResponse {
   string  pong =1;
}

```


##### 2. 使用 *protoc* 生成 *go* 代码

```powershell
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    pb/PingService.proto

```


##### 3. 添加 elsa 依赖

```powershell 
go get github.com/busgo/elsa

```

##### 4. 编写 PingService 服务端

```go 

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/busgo/elsa"
	"github.com/busgo/elsa/pb"
	"github.com/busgo/elsa/pkg/log"
	"google.golang.org/grpc"
)

type PingService struct {
	elsa.Service
	pb.UnimplementedPingServiceServer
}

func (srv *PingService) Ping(ctx context.Context, request *pb.PingRequest) (*pb.PingResponse, error) {

	log.Infof("the PingService receive: %+v", request)
	return &pb.PingResponse{
		Pong: "pong",
	}, nil
}

func (srv *PingService) Build(server *grpc.Server) error {
	pb.RegisterPingServiceServer(server, srv)
	return nil
}

func (srv *PingService) Export() string {
	return pb.PingService_ServiceDesc.ServiceName
}


func main() {

	app, err := elsa.New().
		Name("ping-service").
		RegistryUrl("etcd://127.0.0.1:2379?dail_timeout=5&session_ttl=5&retry_period=5").
		Services(new(PingService)).
		Protocol("grpc").
		BalancePolicy("round_robin").
		Port(8001).
		Export()

	if err != nil {
		panic(err)
	}

	defer app.Close()

	go func() {
		if err = app.Run(); err != nil {
			panic(err)
		}
	}()
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	<-signalCh

}

```


##### 5. 编写 PingService 客户端

```go 

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/busgo/elsa"
	"github.com/busgo/elsa/pb"
	"github.com/busgo/elsa/pkg/log"
	"google.golang.org/grpc"
)

type PingClients struct {
	elsa.Reference
	pb.PingServiceClient
}

func (clients *PingClients) Build(conn *grpc.ClientConn) error {

	clients.PingServiceClient = pb.NewPingServiceClient(conn)
	return nil
}
func (clients *PingClients) Ref() string {
	return pb.PingService_ServiceDesc.ServiceName
}

func main() {

	clients := new(PingClients)

	app, err := elsa.New().
		Name("ping-clients").
		BalancePolicy("round_robin").
		RegistryUrl("etcd://127.0.0.1:2379?dail_timeout=5&ttl=5&retry_period=5").
		Refs(clients).
		Protocol("grpc").
		Port(8002).
		Export()

	if err != nil {
		panic(err)
	}

	defer app.Close()

	go func() {
		if err := app.Run(); err != nil {
			panic(err)
		}
	}()

	go func() {

		for {

			time.Sleep(time.Second)

			reponse, err := clients.PingServiceClient.Ping(context.TODO(), &pb.PingRequest{
				Ping: "Ping",
			})
			if err != nil {
				log.Infof("Failed to call PingServiceClient.Ping, cause: %s", err.Error())
				continue
			}
			log.Infof("call PingServiceClient.Ping success ---> %+v", reponse)

		}
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	<-signalCh
}

``` 

##### 6. 启动应用

###### 1. 启动 *etcd* 服务

```powershell 

  rm -rf /tmp/etcd-data.tmp && mkdir -p /tmp/etcd-data.tmp && \
  docker rmi gcr.io/etcd-development/etcd:v3.4.21 || true && \
  docker run \
  -p 2379:2379 \
  -p 2380:2380 \
  --mount type=bind,source=/tmp/etcd-data.tmp,destination=/etcd-data \
  --name etcd-gcr-v3.4.21 \
  gcr.io/etcd-development/etcd:v3.4.21 \
  /usr/local/bin/etcd \
  --name s1 \
  --data-dir /etcd-data \
  --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://0.0.0.0:2379 \
  --listen-peer-urls http://0.0.0.0:2380 \
  --initial-advertise-peer-urls http://0.0.0.0:2380 \
  --initial-cluster s1=http://0.0.0.0:2380 \
  --initial-cluster-token tkn \
  --initial-cluster-state new \
  --log-level info \
  --logger zap \
  --log-outputs stderr

```

###### 2. 启动 PingService 服务端应用

```powershell 

  go mod tidy  

  go run server.go 

```


###### 3. 启动 PingService 客户端应用

```powershell 

  go mod tidy  

  go run client.go 

```


### 联系我

![加好友](docs/qr-code.jpeg)

### 贡献

欢迎参与项目贡献！提交PR修复 *bug* 或者新建 [issues](https://github.com/busgo/elsa/issues)


###  License

elsa is under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.
