package cmd

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	server "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/uswitch/yggdrasil/pkg/envoy"
	"github.com/uswitch/yggdrasil/pkg/k8s"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type clusterConfig struct {
	APIServer string `json:"apiServer"`
	Ca        string `json:"ca"`
	Token     string `json:"token"`
	TokenPath string `json:"tokenPath"`
}

type config struct {
	IngressClass               string                    `json:"ingressClass"`
	NodeName                   string                    `json:"nodeName"`
	Clusters                   []clusterConfig           `json:"clusters"`
	Certificates               []envoy.Certificate       `json:"certificates"`
	TrustCA                    string                    `json:"trustCA"`
	UpstreamPort               uint32                    `json:"upstreamPort"`
	EnvoyListenerIpv4Address   string                    `json:"envoyListenerIpv4Address"`
	EnvoyPort                  uint32                    `json:"envoyPort"`
	MaxEjectionPercentage      uint32                    `json:"maxEjectionPercentage"`
	HostSelectionRetryAttempts int64                     `json:"hostSelectionRetryAttempts"`
	UpstreamHealthCheck        envoy.UpstreamHealthCheck `json:"upstreamHealthCheck"`
	UseRemoteAddress           bool                      `json:"useRemoteAddress"`
	HttpExtAuthz               envoy.HttpExtAuthz        `json:"httpExtAuthz"`
	HttpGrpcLogger             envoy.HttpGrpcLogger      `json:"httpGrpcLogger"`
	AlpnProtocols              []string                  `json:"alpnProtocols"`
}

// Hasher returns node ID as an ID
type Hasher struct {
}

var (
	cfgFile    string
	kubeConfig []string
)

var rootCmd = &cobra.Command{
	Use:   "yggdrasil",
	Short: "yggdrasil creates an envoy control plane that watches ingress objects  ",
	RunE:  main,
}

//Execute runs the function
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")
	rootCmd.PersistentFlags().String("address", "0.0.0.0:8080", "yggdrasil envoy control plane listen address")
	rootCmd.PersistentFlags().String("health-address", "0.0.0.0:8081", "yggdrasil health API listen address")
	rootCmd.PersistentFlags().String("node-name", "", "envoy node name")
	rootCmd.PersistentFlags().String("cert", "", "certfile")
	rootCmd.PersistentFlags().String("key", "", "keyfile")
	rootCmd.PersistentFlags().String("ca", "", "trustedCA")
	rootCmd.PersistentFlags().StringSlice("ingress-classes", nil, "Ingress classes to watch")
	rootCmd.PersistentFlags().StringArrayVar(&kubeConfig, "kube-config", nil, "Path to kube config")
	rootCmd.PersistentFlags().Bool("debug", false, "Log at debug level")
	rootCmd.PersistentFlags().Uint32("upstream-port", 443, "port used to connect to the upstream ingresses")
	rootCmd.PersistentFlags().String("envoy-listener-ipv4-address", "0.0.0.0", "IPv4 address by the envoy proxy to accept incoming connections")
	rootCmd.PersistentFlags().Uint32("envoy-port", 10000, "port by the envoy proxy to accept incoming connections")
	rootCmd.PersistentFlags().Int32("max-ejection-percentage", -1, "maximal percentage of hosts ejected via outlier detection. Set to >=0 to activate outlier detection in envoy.")
	rootCmd.PersistentFlags().Int64("host-selection-retry-attempts", -1, "Number of host selection retry attempts. Set to value >=0 to enable")
	rootCmd.PersistentFlags().String("retry-on", "5xx", "default comma-separated list of retry policies")
	rootCmd.PersistentFlags().Duration("upstream-healthcheck-interval", 10*time.Second, "duration of the upstream health check interval")
	rootCmd.PersistentFlags().Duration("upstream-healthcheck-timeout", 5*time.Second, "timeout of the upstream healthchecks")
	rootCmd.PersistentFlags().Uint32("upstream-healthcheck-healthy", 3, "number of successful healthchecks before the backend is considered healthy")
	rootCmd.PersistentFlags().Uint32("upstream-healthcheck-unhealthy", 3, "number of failed healthchecks before the backend is considered unhealthy")
	rootCmd.PersistentFlags().Bool("use-remote-address", false, "populates the X-Forwarded-For header with the client address. Set to true when used as edge proxy")
	rootCmd.PersistentFlags().String("http-grpc-logger-name", "", "Name of the access log")
	rootCmd.PersistentFlags().String("http-grpc-logger-cluster", "", "The name of the upstream gRPC cluster")
	rootCmd.PersistentFlags().Duration("http-grpc-logger-timeout", 200*time.Millisecond, "The timeout for the gRPC request")
	rootCmd.PersistentFlags().StringSlice("http-grpc-logger-request-headers", []string{}, "access logs request headers")
	rootCmd.PersistentFlags().StringSlice("http-grpc-logger-response-headers", []string{}, "access logs response headers")
	rootCmd.PersistentFlags().String("http-ext-authz-cluster", "", "The name of the upstream gRPC cluster")
	rootCmd.PersistentFlags().Duration("http-ext-authz-timeout", 200*time.Millisecond, "The timeout for the gRPC request. This is the timeout for a specific request.")
	rootCmd.PersistentFlags().Uint32("http-ext-authz-max-request-bytes", 8192, "Sets the maximum size of a message body that the filter will hold in memory")
	rootCmd.PersistentFlags().Bool("http-ext-authz-allow-partial-message", true, "When this field is true, Envoy will buffer the message until max_request_bytes is reached")
	rootCmd.PersistentFlags().Bool("http-ext-authz-pack-as-bytes", false, "When this field is true, Envoy will send the body as raw bytes.")
	rootCmd.PersistentFlags().Bool("http-ext-authz-failure-mode-allow", true, "Changes filters behaviour on errors")
	rootCmd.PersistentFlags().StringSlice("alpn-protocols", []string{}, "exposed listener ALPN protocols")
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("address", rootCmd.PersistentFlags().Lookup("address"))
	viper.BindPFlag("healthAddress", rootCmd.PersistentFlags().Lookup("health-address"))
	viper.BindPFlag("nodeName", rootCmd.PersistentFlags().Lookup("node-name"))
	viper.BindPFlag("ingressClasses", rootCmd.PersistentFlags().Lookup("ingress-classes"))
	viper.BindPFlag("cert", rootCmd.PersistentFlags().Lookup("cert"))
	viper.BindPFlag("key", rootCmd.PersistentFlags().Lookup("key"))
	viper.BindPFlag("trustCA", rootCmd.PersistentFlags().Lookup("ca"))
	viper.BindPFlag("upstreamPort", rootCmd.PersistentFlags().Lookup("upstream-port"))
	viper.BindPFlag("envoyListenerIpv4Address", rootCmd.PersistentFlags().Lookup("envoy-listener-ipv4-address"))
	viper.BindPFlag("envoyPort", rootCmd.PersistentFlags().Lookup("envoy-port"))
	viper.BindPFlag("maxEjectionPercentage", rootCmd.PersistentFlags().Lookup("max-ejection-percentage"))
	viper.BindPFlag("hostSelectionRetryAttempts", rootCmd.PersistentFlags().Lookup("host-selection-retry-attempts"))
	viper.BindPFlag("retryOn", rootCmd.PersistentFlags().Lookup("retry-on"))
	viper.BindPFlag("upstreamHealthCheck.interval", rootCmd.PersistentFlags().Lookup("upstream-healthcheck-interval"))
	viper.BindPFlag("upstreamHealthCheck.timeout", rootCmd.PersistentFlags().Lookup("upstream-healthcheck-timeout"))
	viper.BindPFlag("upstreamHealthCheck.healthyThreshold", rootCmd.PersistentFlags().Lookup("upstream-healthcheck-healthy"))
	viper.BindPFlag("upstreamHealthCheck.unhealthyThreshold", rootCmd.PersistentFlags().Lookup("upstream-healthcheck-unhealthy"))
	viper.BindPFlag("useRemoteAddress", rootCmd.PersistentFlags().Lookup("use-remote-address"))
	viper.BindPFlag("httpGrpcLogger.name", rootCmd.PersistentFlags().Lookup("http-grpc-logger-name"))
	viper.BindPFlag("httpGrpcLogger.cluster", rootCmd.PersistentFlags().Lookup("http-grpc-logger-cluster"))
	viper.BindPFlag("httpGrpcLogger.timeout", rootCmd.PersistentFlags().Lookup("http-grpc-logger-timeout"))
	viper.BindPFlag("httpGrpcLogger.requestHeaders", rootCmd.PersistentFlags().Lookup("http-grpc-logger-request-headers"))
	viper.BindPFlag("httpGrpcLogger.responseHeaders", rootCmd.PersistentFlags().Lookup("http-grpc-logger-response-headers"))
	viper.BindPFlag("httpExtAuthz.cluster", rootCmd.PersistentFlags().Lookup("http-ext-authz-cluster"))
	viper.BindPFlag("httpExtAuthz.timeout", rootCmd.PersistentFlags().Lookup("http-ext-authz-timeout"))
	viper.BindPFlag("httpExtAuthz.maxRequestBytes", rootCmd.PersistentFlags().Lookup("http-ext-authz-max-request-bytes"))
	viper.BindPFlag("httpExtAuthz.allowPartialMessage", rootCmd.PersistentFlags().Lookup("http-ext-authz-allow-partial-message"))
	viper.BindPFlag("httpExtAuthz.packAsBytes", rootCmd.PersistentFlags().Lookup("http-ext-authz-pack-as-bytes"))
	viper.BindPFlag("httpExtAuthz.FailureModeAllow", rootCmd.PersistentFlags().Lookup("http-ext-authz-failure-mode-allow"))
	viper.BindPFlag("alpnProtocols", rootCmd.PersistentFlags().Lookup("alpn-protocols"))
}

func initConfig() {
	// Don't forget to read config either from cfgFile or from home directory!
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
		if err := viper.ReadInConfig(); err != nil {
			fmt.Println("Can't read config:", err)
			os.Exit(1)
		}
	}
}

func main(*cobra.Command, []string) error {
	flag.Set("logtostderr", "true")
	var c config
	err := viper.Unmarshal(&c)
	if err != nil {
		return fmt.Errorf("error unmarshalling viper config: %s", err)
	}

	if !envoy.ValidateEnvoyRetryOn(viper.GetString("retryOn")) {
		return fmt.Errorf("invalid retry-on parameter: %s", viper.GetString("retryOn"))
	}

	if viper.Get("debug") == true {
		log.SetLevel(log.DebugLevel)
	}

	clusterSources, err := createSources(c.Clusters)
	if err != nil {
		return fmt.Errorf("error creating sources: %s", err)
	}

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	configSources, err := configFromKubeConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("error parsing kube config: %s", err)
	}

	sources := append(clusterSources, configSources...)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hash := Hasher{}
	envoyCache := cache.NewSnapshotCache(false, hash, nil)

	err = checkDownStreamTLSSetup(viper.GetString("cert"), viper.GetString("key"))
	if err != nil {
		log.Fatalf("TLS setup failed: %s", err)
	}

	if len(c.Certificates) == 0 && viper.GetString("cert") != "" && viper.GetString("key") != "" {
		c.Certificates = []envoy.Certificate{
			{Hosts: []string{"*"}, Cert: viper.GetString("cert"), Key: viper.GetString("key")},
		}
	}

	// load the certificates from the file system
	for idx, certificate := range c.Certificates {
		certPath := certificate.Cert
		keyPath := certificate.Key

		certBytes, err := ioutil.ReadFile(certPath)
		if err != nil {
			log.Fatalf("Failed to read %s: %v", certPath, err)
		}

		keyBytes, err := ioutil.ReadFile(keyPath)
		if err != nil {
			log.Fatalf("Failed to read %s: %v", keyPath, err)
		}

		c.Certificates[idx].Cert = string(certBytes)
		c.Certificates[idx].Key = string(keyBytes)
	}
	aggregator := k8s.NewAggregator(sources, ctx)
	configurator := envoy.NewKubernetesConfigurator(
		viper.GetString("nodeName"),
		c.Certificates,
		viper.GetString("trustCA"),
		viper.GetStringSlice("ingressClasses"),
		envoy.WithUpstreamPort(uint32(viper.GetInt32("upstreamPort"))),
		envoy.WithEnvoyListenerIpv4Address(viper.GetString("envoyListenerIpv4Address")),
		envoy.WithEnvoyPort(uint32(viper.GetInt32("envoyPort"))),
		envoy.WithOutlierPercentage(viper.GetInt32("maxEjectionPercentage")),
		envoy.WithHostSelectionRetryAttempts(viper.GetInt64("hostSelectionRetryAttempts")),
		envoy.WithUpstreamHealthCheck(c.UpstreamHealthCheck),
		envoy.WithUseRemoteAddress(c.UseRemoteAddress),
		envoy.WithHttpExtAuthzCluster(c.HttpExtAuthz),
		envoy.WithHttpGrpcLogger(c.HttpGrpcLogger),
		envoy.WithDefaultRetryOn(viper.GetString("retryOn")),
		envoy.WithAlpnProtocols(viper.GetStringSlice("alpnProtocols")),
	)
	snapshotter := envoy.NewSnapshotter(envoyCache, configurator, aggregator)

	go snapshotter.Run(aggregator)
	go aggregator.Run()

	envoyServer := server.NewServer(ctx, envoyCache, &callbacks{})
	go runEnvoyServer(envoyServer, viper.GetString("address"), viper.GetString("healthAddress"), ctx.Done())

	<-stopCh
	return nil
}

// checkDownStreamTLSSetup if only one of the two values is set.
func checkDownStreamTLSSetup(cert string, key string) error {
	errorStringPattern := "only '%s' flag is specified. To enable TLS, specify both 'cert' and 'key'"
	if cert == "" && key != "" {
		return fmt.Errorf(errorStringPattern, "key")
	}
	if cert != "" && key == "" {
		return fmt.Errorf(errorStringPattern, "cert")
	}
	return nil
}

func createClientConfig(path string) (*rest.Config, error) {
	if path == "" {
		return rest.InClusterConfig()
	}
	return clientcmd.BuildConfigFromFlags("", path)
}

func createSources(clusters []clusterConfig) ([]*kubernetes.Clientset, error) {
	sources := []*kubernetes.Clientset{}

	for _, cluster := range clusters {

		var token string

		if cluster.TokenPath != "" {
			bytes, err := ioutil.ReadFile(cluster.TokenPath)
			if err != nil {
				return sources, err
			}
			token = string(bytes)
		} else {
			token = cluster.Token
		}

		config := &rest.Config{
			BearerToken: token,
			Host:        cluster.APIServer,
			TLSClientConfig: rest.TLSClientConfig{
				CAFile: cluster.Ca,
			},
		}
		clientSet, err := kubernetes.NewForConfig(config)
		if err != nil {
			return sources, err
		}
		sources = append(sources, clientSet)
	}

	return sources, nil
}

func configFromKubeConfig(paths []string) ([]*kubernetes.Clientset, error) {
	sources := []*kubernetes.Clientset{}

	for _, configPath := range paths {
		config, err := createClientConfig(configPath)
		if err != nil {
			return sources, err
		}
		clientSet, err := kubernetes.NewForConfig(config)
		if err != nil {
			return sources, err
		}
		sources = append(sources, clientSet)
	}

	return sources, nil
}

// ID function
func (h Hasher) ID(node *core.Node) string {
	if node == nil {
		return "unknown"
	}
	return node.Id
}
