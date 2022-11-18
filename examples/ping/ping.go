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

type PingService struct {
	elsa.Service
	pb.UnimplementedPingServiceServer
}

func (srv *PingService) Ping(ctx context.Context, request *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{
		Pong: "pong--->" + request.Ping,
	}, nil
}

func (srv *PingService) Build(s *grpc.Server) error {
	pb.RegisterPingServiceServer(s, srv)
	return nil
}

func (srv *PingService) Export() string {
	return pb.PingService_ServiceDesc.ServiceName
}

type TradeClients struct {
	pb.TradeServiceClient
	elsa.Reference
}

func (clients *TradeClients) Build(cc *grpc.ClientConn) error {
	clients.TradeServiceClient = pb.NewTradeServiceClient(cc)
	return nil
}

func (clients *TradeClients) Ref() string {
	return pb.TradeService_ServiceDesc.ServiceName
}

func main() {

	pingService := new(PingService)
	tradeClients := new(TradeClients)

	app := elsa.New().Name("ping_service").
		Version("1.0").
		Port(8001).
		RegistryUrl("etcd://127.0.0.1:2379?dial_timeout=5&ttl=10").
		Services(pingService).
		Refs(tradeClients)

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
			response, err := tradeClients.Trade(context.TODO(), &pb.TradeRequest{UserId: 1})
			if err != nil {

				log.Errorf("trade service client invoke error %s", err.Error())
				continue
			}

			log.Infof("trade service client receive ---> %d", response.OrderId)

		}
	}()

	c := make(chan os.Signal, 1)

	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

}
