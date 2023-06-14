// Package env get env & app config, all the public field must after init()
// finished and flag.Parse().
package env

import (
	"flag"
	"os"
)

// deploy env.
const (
	DeployEnvDev    = "dev"
	DeployEnvFat1   = "fat1"
	DeployEnvUat    = "uat"
	DeployEnvAvalon = "avalon"
	DeployEnvPre    = "pre"
	DeployEnvProd   = "prod"
)

// env default value.
const (
	// env
	_region    = "sh"
	_zone      = "sh001"
	_deployEnv = "dev"
)

// env configuration.
var (
	// Hostname machine hostname.
	Hostname string
	// DeployEnv deploy env where app at.
	DeployEnv string

	// AppID is global unique application id, register by service tree.
	// such as main.arch.disocvery.
	AppID string
	// DiscoveryAppID global unique application id for disocvery, register by service tree.
	// such as main.arch.disocvery.
	//DiscoveryAppID string
	// DiscoveryZone is discovery zone.
	//DiscoveryZone string
	// DiscoveryHost is discovery host.
	//DiscoveryHost string
	// IP FIXME(haoguanwei) #240
	//IP = os.Getenv("POD_IP")
)

// app default value.
const (
	_httpPort  = "8000"
	_gorpcPort = "8099"
	_grpcPort  = "9000"
)

// app configraution.
var (
// HTTPPort app listen http port.
//HTTPPort string
// GORPCPort app listen gorpc port.
//GORPCPort string
// GRPCPort app listen grpc port.
//GRPCPort string
)

func init() {
	var err error
	if Hostname, err = os.Hostname(); err != nil || Hostname == "" {
		Hostname = os.Getenv("HOSTNAME")
	}

	addFlag(flag.CommandLine)
}

func addFlag(fs *flag.FlagSet) {
	// env
	//fs.StringVar(&Region, "region", defaultString("REGION", _region), "avaliable region. or use REGION env variable, value: sh etc.")
	//fs.StringVar(&Zone, "zone", defaultString("ZONE", _zone), "avaliable zone. or use ZONE env variable, value: sh001/sh002 etc.")
	//fs.StringVar(&DeployEnv, "deploy.env", defaultString("DEPLOY_ENV", _deployEnv), "deploy env. or use DEPLOY_ENV env variable, value: dev/fat1/uat/pre/prod etc.")
	fs.StringVar(&AppID, "appid", os.Getenv("APP_ID"), "appid is global unique application id, register by service tree. or use APP_ID env variable.")
	//fs.StringVar(&Color, "deploy.color", os.Getenv("DEPLOY_COLOR"), "deploy.color is the identification of different experimental group.")
	// discovery
	//fs.StringVar(&DiscoveryAppID, "discovery.appid", os.Getenv("DISCOVERY_APPID"), "dsappid is global unique application id for disocvery, register by service tree. or use DISCOVERY_APPID env variable.")
	//fs.StringVar(&DiscoveryZone, "discovery.zone", os.Getenv("DISCOVERY_ZONE"), "discovery zone. sh001/sh002 etc.")
	//fs.StringVar(&DiscoveryHost, "discovery.host", os.Getenv("DISCOVERY_HOST"), "discovery host. discovery.biilbili.co etc.")
	// app
	//fs.StringVar(&HTTPPort, "http.port", defaultString("DISCOVERY_HTTP_PORT", _httpPort), "app listen http port, default: 8000")
	//fs.StringVar(&GORPCPort, "gorpc.port", defaultString("DISCOVERY_GORPC_PORT", _gorpcPort), "app listen gorpc port, default: 8099")
	//fs.StringVar(&GRPCPort, "grpc.port", defaultString("DISCOVERY_GRPC_PORT", _grpcPort), "app listen grpc port, default: 9000")
}

func defaultString(env, value string) string {
	v := os.Getenv(env)
	if v == "" {
		return value
	}
	return v
}
