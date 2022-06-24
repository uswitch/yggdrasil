package k8s

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type IngressLister interface {
	List() ([]v1beta1.Ingress, error)
}

type Aggregator struct {
	factories     []*informers.SharedInformerFactory
	events        chan SyncDataEvent
	ingressStores []cache.Store
}

func (a *Aggregator) Events() chan SyncDataEvent {
	return a.events
}

// NewAggregator returns a new Aggregator initialized with resource informers
func NewAggregator(k8sClients []*kubernetes.Clientset, ctx context.Context) *Aggregator {
	a := Aggregator{
		events:        make(chan SyncDataEvent, watch.DefaultChanSize),
		ingressStores: []cache.Store{},
	}
	informersSynced := []cache.InformerSynced{}

	for _, c := range k8sClients {
		factory := informers.NewSharedInformerFactory(c, time.Minute)

		ingressInformer := getIngressInformer(factory, c)
		a.EventsIngresses(ctx, ingressInformer)
		a.ingressStores = append(a.ingressStores, ingressInformer.GetStore())

		a.factories = append(a.factories, &factory)
		informersSynced = append(informersSynced, ingressInformer.HasSynced)
	}

	if !cache.WaitForCacheSync(ctx.Done(), informersSynced...) {
		logrus.Panicf("Unable to populate caches")
	}
	return &a
}

func getIngressInformer(factory informers.SharedInformerFactory, clientSet *kubernetes.Clientset) (ingressInformer cache.SharedIndexInformer) {
	for _, apiGroup := range []string{"networking.k8s.io/v1", "networking.k8s.io/v1beta1", "extensions/v1beta1"} {
		resources, err := clientSet.ServerResourcesForGroupVersion(apiGroup)
		if err != nil {
			continue
		}
		for _, rs := range resources.APIResources {
			if rs.Name == "ingresses" {
				switch apiGroup {
				case "networking.k8s.io/v1":
					ingressInformer = factory.Networking().V1().Ingresses().Informer()
				case "networking.k8s.io/v1beta1":
					ingressInformer = factory.Networking().V1beta1().Ingresses().Informer()
				case "extensions/v1beta1":
					ingressInformer = factory.Extensions().V1beta1().Ingresses().Informer()
				}
				logrus.Infof("watching ingress resources of apiGroup %s", apiGroup)
			}
		}
		if ingressInformer != nil {
			break
		}
	}
	return ingressInformer
}

// Run is the synchronization loop
func (a *Aggregator) Run() {
	for {
		a.events <- SyncDataEvent{SyncType: COMMAND}
		time.Sleep(5 * time.Second)
	}
}
