package elsa

import (
	"fmt"
	"strconv"
	"time"

	"github.com/busgo/elsa/limit"
	"github.com/busgo/elsa/pkg/common"
	"github.com/busgo/elsa/pkg/log"
	"github.com/busgo/elsa/pkg/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

// Service
type Service interface {
	Build(*grpc.Server) error

	//
	//    io.busgo.trade.TradeService?version=1.0
	//    io.busgo.trade.TradeService
	//    direct://127.0.0.1:8080/io.busgo.trade.TradeService
	//
	Export() string
}

func (app *application) exportServices() error {

	for _, service := range app.services {

		if err := service.Build(app.srv); err != nil {
			return err
		}

		if err := app.exportService(service.Export()); err != nil {
			return err
		}

	}

	reflection.Register(app.srv)
	return app.exportService(grpc_reflection_v1alpha.ServerReflection_ServiceDesc.ServiceName)
}

// exportService  export a service
func (app *application) exportService(export string) error {

	parameter := map[string]string{
		"application": app.name,
		"version":     app.version,
		"category":    common.DefaultCategory,
	}

	u, err := common.ParseURL(export, parameter)

	if err != nil {
		return err
	}
	u.Host = fmt.Sprintf("%s:%d", util.LocalIp(), app.port)
	u.Parameters["timestamp"] = strconv.Itoa(int(time.Now().Unix()))

	concurrency, err := strconv.ParseInt(u.Parameter("concurrency", "0"), 10, 64)
	if err != nil {
		concurrency = 0
	}

	if concurrency > 0 {
		limit.Use(u.Path, limit.New(concurrency))
	}
	app.registryService.Register(u)

	log.Infof("the service for %s, export success", u.String())
	return nil
}
