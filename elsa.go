package elsa

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	_ "github.com/busgo/elsa/lb/least"
	"github.com/busgo/elsa/pkg/common"
	"github.com/busgo/elsa/pkg/log"
	"github.com/busgo/elsa/plugin"
	"github.com/busgo/elsa/registry"
	"github.com/busgo/elsa/registry/transport"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
)

type Application interface {
	Name(string) Application
	Version(string) Application
	Port(int32) Application
	BalancePolicy(string) Application

	//  etcd://127.0.0.1:2379?backup=127.0.0.1:2389,127.0.0.1:2382&username=root&password=123456&ttl=10&dial_timeout=5
	RegistryUrl(string) Application
	Services(...Service) Application
	Refs(...Reference) Application

	Export() error
	Run() error
	io.Closer
}

type application struct {
	name        string
	version     string
	port        int32
	lb          string
	registryUrl string

	registryService registry.RegistryService
	services        []Service
	refs            []Reference

	srv *grpc.Server

	exported bool
}

func New() Application {

	return &application{
		name:    "elsa",
		version: "1.0",
		port:    8080,
		lb:      "round_robin",
		srv: grpc.NewServer(
			grpc_middleware.WithStreamServerChain(plugin.RateLimitStreamServerInterceptor()),
			grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(plugin.RateLimitUnaryServerInterceptor())),
		),
	}
}

func (app *application) Name(name string) Application {
	app.name = name
	return app
}

func (app *application) Version(version string) Application {
	app.version = version
	return app
}

func (app *application) Port(port int32) Application {
	app.port = port
	return app
}

func (app *application) BalancePolicy(lb string) Application {
	app.lb = lb
	return app
}

func (app *application) RegistryUrl(registryUrl string) Application {
	app.registryUrl = registryUrl
	return app
}

func (app *application) Services(services ...Service) Application {
	app.services = append(app.services, services...)
	return app
}

func (app *application) Refs(refs ...Reference) Application {
	app.refs = append(app.refs, refs...)
	return app
}

func (app *application) Export() error {

	// export registry
	if err := app.exportRegistry(); err != nil {
		return err
	}

	// export service
	if err := app.exportServices(); err != nil {
		return err
	}

	// export refs
	if err := app.exportRefs(); err != nil {
		return err
	}

	app.exported = true
	return nil
}

func (app *application) Run() error {
	if !app.exported {
		return errors.New("the application has not  export, please export")
	}

	if app.port <= 0 {
		app.port = 8080
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", app.port))
	if err != nil {
		return err
	}

	log.Infof("the application %s start listen on %s ", app.name, fmt.Sprintf(":%d", app.port))
	return app.srv.Serve(l)
}

func (app *application) Close() error {

	app.srv.GracefulStop()
	_ = app.registryService.Close()
	return nil
}

// exportRegistry  export registry service center
func (app *application) exportRegistry() error {

	if app.registryUrl == "" {
		return errors.New("the registry url is nil")
	}

	u, err := common.Decode(app.registryUrl)
	if err != nil {
		return err
	}

	switch u.Protocol {

	case "etcd":
		return app.exportEtcdRegistry(u)

	default:
		return fmt.Errorf("the registry url for %s not support", app.registryUrl)
	}

}

// exportEtcdRegistry export etcd  registry
func (app *application) exportEtcdRegistry(u common.URL) error {

	endpoints := make([]string, 0)
	endpoints = append(endpoints, u.Host)

	if u.Parameter("backup", "") != "" {
		endpoints = append(endpoints, strings.Split(u.Parameter("backup", ""), ",")...)
	}

	username := u.Parameter("username", "")
	password := u.Parameter("password", "")

	ttl, err := strconv.ParseInt(u.Parameter("ttl", "10"), 10, 64)
	if err != nil {
		return err
	}

	dialTimeout, err := strconv.ParseInt(u.Parameter("dial_timeout", "5"), 10, 64)
	if err != nil {
		return err
	}

	t, err := transport.New(
		transport.WithEndpoints(endpoints),
		transport.WithUsername(username),
		transport.WithPassword(password),
		transport.WithTTL(ttl),
		transport.WithDialTimeout(dialTimeout),
	)

	if err != nil {
		return err
	}

	retryPeriod, err := strconv.ParseInt(u.Parameter("retry_period", "5"), 10, 64)
	if err != nil {
		return err
	}

	registryService, err := registry.New(
		registry.WithTransport(t),
		registry.WithretryPeriod(retryPeriod),
		registry.WithDialTimeout(dialTimeout),
	)

	if err != nil {
		return err
	}

	app.registryService = registryService
	log.Infof("the application registry url for %s, export success", app.registryUrl)
	return nil
}
