package main

import (
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"

	"github.com/foogod/go-powerwall"
)

const (
	exporterName = "powerwall"
	exporterVersion = "0.1.0"
	projectURL = "https://github.com/foogod/powerwall_exporter"
	defaultListenAddress = ":9871"
	defaultMetricsPath = "/metrics"
	defaultLoginEmail = "powerwall_exporter@example.org"
)

var options struct {
	Debug bool `long:"debug" description:"Enable debug messages"`
	LogStyle string `long:"log.style" description:"Style of log output to produce" choice:"text" choice:"logfmt" choice:"json" default:"text"`
	ConfigFile string `long:"config.file" description:"Path to config file"`
}

func setOptionDefaults() {
	options.ConfigFile = os.Args[0] + ".yaml"
}

func main() {
	setOptionDefaults()
	_, err := flags.Parse(&options)
	if err != nil {
		os.Exit(1)
	}
	switch options.LogStyle {
	case "text":
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp: true,
			DisableLevelTruncation: true,
			PadLevelText: true,
		})
	case "logfmt":
		log.SetFormatter(&log.TextFormatter{
			DisableColors: true,
			FullTimestamp: true,
		})
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
	}
	if options.Debug {
		log.SetLevel(log.DebugLevel)
	}

	powerwall.SetLogFunc(pwclientLog)

	log.WithFields(log.Fields{"version": exporterVersion}).Infof("Starting %s exporter", exporterName)

	loadConfig(options.ConfigFile)

	startServer()
}

func pwclientLog(v ...interface{}) {
        log.Debug(v...)
}

type Config struct {
	Web WebConfig
	Device DeviceConfig
}
type WebConfig struct {
	ListenAddress string `yaml:"listen-address"`
	MetricsPath string `yaml:"metrics-path"`
}
type DeviceConfig struct {
	GatewayAddress string `yaml:"gateway-address"`
	LoginEmail string `yaml:"login-email"`
	LoginPassword string `yaml:"login-password"`
}

var config Config

func loadConfig(filename string) {
	absPath, err := filepath.Abs(filename)
	if err != nil {
		panic(err)
	}
	log.WithFields(log.Fields{"file": absPath}).Info("Loading config")
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("Unable to read config file: %s", err)
	}

	// Set defaults
	config.Web = WebConfig{
		ListenAddress: defaultListenAddress,
		MetricsPath: defaultMetricsPath,
	}
	config.Device = DeviceConfig{
		LoginEmail: defaultLoginEmail,
	}

	err = yaml.UnmarshalStrict(yamlFile, &config)
	if err != nil {
		log.Fatalf("Unable to parse config file: %s", err)
	}

	// Check required fields
	if config.Device.GatewayAddress == "" {
		log.Fatal("Required parameter device.teg-address not specified in config file")
	}
	if config.Device.LoginPassword == "" {
		log.Fatal("Required parameter device.login-password not specified in config file")
	}
}

func startServer() {
	http.HandleFunc("/", indexPageHandler)

	pwclient := powerwall.NewClient(config.Device.GatewayAddress, config.Device.LoginEmail, config.Device.LoginPassword)

	reg := prometheus.NewRegistry()
	reg.MustRegister(NewPowerwallCollector(pwclient))
	regLogger := log.New()
	regLogger.Level = log.ErrorLevel
	regHandler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		ErrorLog:      regLogger,
		ErrorHandling: promhttp.ContinueOnError,
	})
	http.Handle(config.Web.MetricsPath, regHandler)

	log.WithFields(log.Fields{"listen_address": config.Web.ListenAddress, "metrics_path": config.Web.MetricsPath}).Info("Listening for HTTP connections")
	log.Fatal(http.ListenAndServe(config.Web.ListenAddress, nil))
}

func indexPageHandler(w http.ResponseWriter, r *http.Request) {
	templateValues := struct{
		Exporter string
		Version string
		MetricsPath string
		ProjectURL string
	}{exporterName, exporterVersion, config.Web.MetricsPath, projectURL}

	// Ordinarily we should probably parse the template once ahead of time and
	// reuse it, but people aren't likely to be calling this page over and over
	// again normally, so this is fine for this case.
	t, err := template.New("indexHTML").Parse(indexHTML)
	if err != nil {
		log.Errorf("Error parsing template for index HTML (/): %s", err)
		http.Error(w, "Internal server error", 500)
		return
	}

	err = t.Execute(w, templateValues)
	if err != nil {
		log.Errorf("Error executing template for index HTML (/): %s", err)
	}
}

const indexHTML = `<!doctype html>
<html>
<head>
        <meta charset="UTF-8">
        <title>{{ .Exporter }} exporter</title>
</head>
<body>
        <h1>{{ .Exporter }} exporter for Prometheus (Version {{ .Version }})</h1>
        <p>Exported metrics are available at <a href="{{ .MetricsPath }}">{{ .MetricsPath }}</a></p>
        <h2>More information:</h2>
        <p><a href="{{ .ProjectURL }}">{{ .ProjectURL }}</a></p>
</body>
</html>
`
