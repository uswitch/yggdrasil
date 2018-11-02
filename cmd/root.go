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
	IngressClass string          `json:"ingressClass"`
	NodeName     string          `json:"nodeName"`
	Clusters     []clusterConfig `json:"clusters"`
	Cert         string          `json:"cert"`
	Key          string          `json:"key"`
	TrustCA      string          `json:"trustCA"`
}

// Hasher returns node ID as an ID
type Hasher struct {
}

var (
	cfgFile    string
	sources    []k8scache.ListerWatcher
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
	rootCmd.PersistentFlags().String("node-name", "", "envoy node name")
	rootCmd.PersistentFlags().String("cert", "", "certfile")
	rootCmd.PersistentFlags().String("key", "", "keyfile")
	rootCmd.PersistentFlags().String("ca", "", "trustedCA")
	rootCmd.PersistentFlags().StringSlice("ingress-classes", nil, "Ingress classes to watch")
	rootCmd.PersistentFlags().StringArrayVar(&kubeConfig, "kube-config", nil, "Path to kube config")
	rootCmd.PersistentFlags().Bool("debug", false, "Log at debug level")
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("nodeName", rootCmd.PersistentFlags().Lookup("node-name"))
	viper.BindPFlag("ingressClasses", rootCmd.PersistentFlags().Lookup("ingress-classes"))
	viper.BindPFlag("cert", rootCmd.PersistentFlags().Lookup("cert"))
	viper.BindPFlag("key", rootCmd.PersistentFlags().Lookup("key"))
	viper.BindPFlag("trustCA", rootCmd.PersistentFlags().Lookup("ca"))
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

	err = createSources(c.Clusters)
	if err != nil {
		return fmt.Errorf("error creating sources: %s", err)
	}

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	err = configFromKubeConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("error parsing kube config: %s", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hash := Hasher{}
	envoyCache := cache.NewSnapshotCache(false, hash, nil)

	lister := k8s.NewIngressAggregator(sources)
	configurator := envoy.NewKubernetesConfigurator(
		lister,
		viper.GetString("nodeName"),
		viper.GetString("cert"),
		viper.GetString("key"),
		viper.GetString("trustCA"),
		viper.GetStringSlice("ingressClasses"))
	snapshotter := envoy.NewSnapshotter(envoyCache, configurator, lister.Events())
	go snapshotter.Run(ctx)
	lister.Run(ctx)

	envoyServer := server.NewServer(envoyCache, &callbacks{})
	go runEnvoyServer(envoyServer, ctx.Done())

	<-stopCh
	return nil
}

func createClientConfig(path string) (*rest.Config, error) {
	if path == "" {
		return rest.InClusterConfig()
	}
	return clientcmd.BuildConfigFromFlags("", path)
}

func createSources(clusters []clusterConfig) error {
	for _, cluster := range clusters {

		var token string

		if cluster.TokenPath != "" {
			bytes, err := ioutil.ReadFile(cluster.TokenPath)
			if err != nil {
				return err
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
			return err
		}
		sources = append(sources, k8s.NewListWatch(clientSet))
	}
	return nil
}

func configFromKubeConfig(paths []string) error {
	for _, configPath := range paths {
		config, err := createClientConfig(configPath)
		if err != nil {
			return err
		}
		clientSet, err := kubernetes.NewForConfig(config)
		if err != nil {
			return err
		}
		sources = append(sources, k8s.NewListWatch(clientSet))
	}
	return nil
}

// ID function
func (h Hasher) ID(node *core.Node) string {
	if node == nil {
		return "unknown"
	}
	return node.Id
}
