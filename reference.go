package elsa

import (
	"fmt"
	"strconv"
	"time"

	"github.com/busgo/elsa/pkg/util"

	"github.com/busgo/elsa/discovery"
	"github.com/busgo/elsa/pkg/common"
	"github.com/busgo/elsa/pkg/log"
	"google.golang.org/grpc"
)

type Reference interface {
	Build(*grpc.ClientConn) error

	Ref() string
}

func (app *application) exportRefs() error {
	for _, ref := range app.refs {
		if err := app.exportRef(ref); err != nil {
			return err
		}
	}
	return nil
}

func (app *application) exportRef(ref Reference) error {

	parameter := map[string]string{
		"application": app.name,
		"version":     app.version,
		"category":    "consumers",
		"timestamp":   strconv.Itoa(int(time.Now().Unix())),
	}

	u, err := common.ParseURL(ref.Ref(), parameter)
	if err != nil {
		return err
	}
	u.Host = fmt.Sprintf("%s:%d", util.LocalIp(), app.port)
	lb := u.Parameter("balance_policy", app.lb)
	conn, err := grpc.Dial(
		u.String(),
		grpc.WithInsecure(),
		grpc.WithResolvers(discovery.New(u.Protocol, app.registryService)),
		grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"LoadBalancingPolicy":"%s"}`, lb)),
	)

	if err != nil {
		return err
	}

	if err = ref.Build(conn); err != nil {
		return err
	}
	app.registryService.Register(u)
	log.Infof("the reference service for %s, export success", u.String())
	return nil
}
