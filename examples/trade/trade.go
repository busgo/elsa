package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/busgo/elsa"
	"github.com/busgo/elsa/examples/pb"
	"github.com/busgo/elsa/pkg/log"
	"google.golang.org/grpc"
)

type TradeService struct {
	elsa.Service
	pb.UnimplementedTradeServiceServer
}

func (srv *TradeService) Trade(ctx context.Context, request *pb.TradeRequest) (*pb.TradeResponse, error) {
	return &pb.TradeResponse{
		OrderId: 100,
	}, nil
}

func (srv *TradeService) Build(s *grpc.Server) error {
	pb.RegisterTradeServiceServer(s, srv)
	return nil
}

func (srv *TradeService) Export() string {
	return pb.TradeService_ServiceDesc.ServiceName
}

type PingClients struct {
	pb.PingServiceClient
	elsa.Reference
}

func (clients *PingClients) Build(cc *grpc.ClientConn) error {
	clients.PingServiceClient = pb.NewPingServiceClient(cc)
	return nil
}

func (clients *PingClients) Ref() string {
	return pb.PingService_ServiceDesc.ServiceName
}

func main() {

	tradeService := new(TradeService)
	pingClients := new(PingClients)

	app := elsa.New().Name("trade_service").
		Version("1.0").
		Port(8002).
		RegistryUrl("etcd://127.0.0.1:2379?dial_timeout=5&ttl=10").
		Services(tradeService).
		Refs(pingClients)

	if err := app.Export(); err != nil {
		log.Infof(err.Error())
	}

	go func() {
		if err := app.Run(); err != nil {
			panic(err)
		}
	}()

	defer app.Close()

	go func() {

		for {
			time.Sleep(time.Second)
			response, err := pingClients.Ping(context.TODO(), &pb.PingRequest{Ping: "Ping"})
			if err != nil {
				continue
			}

			log.Infof("ping service receive ---> %s", response.Pong)

		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

}
