package cmd

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/envoyproxy/go-control-plane/pkg/server"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/uswitch/yggdrasil/pkg/envoy"
	"github.com/uswitch/yggdrasil/pkg/k8s"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	k8scache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type clusterConfig struct {
	APIServer string `json:"apiServer"`
	Ca        string `json:"ca"`
	Token     string `json:"token"`
	TokenPath string `json:"tokenPath"`
}

type config struct {
	IngressClass string              `json:"ingressClass"`
	NodeName     string              `json:"nodeName"`
	Clusters     []clusterConfig     `json:"clusters"`
	Certificates []envoy.Certificate `json:"certificates"`
	TrustCA      string              `json:"trustCA"`
	UpstreamPort uint32              `json:"upstreamPort"`
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
	Short: "yggdrasil creates an envoy control plane that watches ingress objects",
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
	rootCmd.PersistentFlags().Uint32("envoy-port", 443, "port by the envoy proxy to accept incoming connections")
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("address", rootCmd.PersistentFlags().Lookup("address"))
	viper.BindPFlag("healthAddress", rootCmd.PersistentFlags().Lookup("health-address"))
	viper.BindPFlag("nodeName", rootCmd.PersistentFlags().Lookup("node-name"))
	viper.BindPFlag("ingressClasses", rootCmd.PersistentFlags().Lookup("ingress-classes"))
	viper.BindPFlag("cert", rootCmd.PersistentFlags().Lookup("cert"))
	viper.BindPFlag("key", rootCmd.PersistentFlags().Lookup("key"))
	viper.BindPFlag("trustCA", rootCmd.PersistentFlags().Lookup("ca"))
	viper.BindPFlag("upstreamPort", rootCmd.PersistentFlags().Lookup("upstream-port"))
	viper.BindPFlag("envoyPort", rootCmd.PersistentFlags().Lookup("envoy-port"))

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

	if len(c.Certificates) == 0 {
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
	lister := k8s.NewIngressAggregator(sources)
	configurator := envoy.NewKubernetesConfigurator(
		viper.GetString("nodeName"),
		c.Certificates,
		viper.GetString("trustCA"),
		viper.GetStringSlice("ingressClasses"),
		envoy.WithUpstreamPort(uint32(viper.GetInt32("upstreamPort"))),
		envoy.WithEnvoyPort(uint32(viper.GetInt32("envoyPort"))))
	snapshotter := envoy.NewSnapshotter(envoyCache, configurator, lister)

	go snapshotter.Run(ctx)
	lister.Run(ctx)

	envoyServer := server.NewServer(envoyCache, &callbacks{})
	go runEnvoyServer(envoyServer, viper.GetString("address"), viper.GetString("healthAddress"), ctx.Done())

	<-stopCh
	return nil
}

func createClientConfig(path string) (*rest.Config, error) {
	if path == "" {
		return rest.InClusterConfig()
	}
	return clientcmd.BuildConfigFromFlags("", path)
}

func createSources(clusters []clusterConfig) ([]k8scache.ListerWatcher, error) {
	sources := []k8scache.ListerWatcher{}

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
		sources = append(sources, k8s.NewListWatch(clientSet))
	}

	return sources, nil
}

func configFromKubeConfig(paths []string) ([]k8scache.ListerWatcher, error) {
	sources := []k8scache.ListerWatcher{}

	for _, configPath := range paths {
		config, err := createClientConfig(configPath)
		if err != nil {
			return sources, err
		}
		clientSet, err := kubernetes.NewForConfig(config)
		if err != nil {
			return sources, err
		}
		sources = append(sources, k8s.NewListWatch(clientSet))
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
