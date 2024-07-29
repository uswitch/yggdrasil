package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type IngressStore struct {
	Store                 cache.Store
	Maintenance           bool
	KubernetesClusterName string
}

type Aggregator struct {
	factories     []*informers.SharedInformerFactory
	events        chan SyncDataEvent
	ingressStores []IngressStore
	secretsStore  []cache.Store
}

func (a *Aggregator) Events() chan SyncDataEvent {
	return a.events
}

func (a *Aggregator) GetSecrets() ([]*v1.Secret, error) {
	allSecrets := make([]*v1.Secret, 0)
	for _, store := range a.secretsStore {
		secrets := store.List()
		for _, obj := range secrets {
			secret, ok := obj.(*v1.Secret)
			if !ok {
				return nil, fmt.Errorf("unexpected object in store: %+v", obj)
			}
			allSecrets = append(allSecrets, secret)
		}
	}
	return allSecrets, nil
}

// NewAggregator returns a new Aggregator initialized with resource informers
func NewAggregator(k8sClients []KubernetesConfig, ctx context.Context, syncSecrets bool) *Aggregator {
	a := Aggregator{
		events:        make(chan SyncDataEvent, watch.DefaultChanSize),
		ingressStores: []IngressStore{},
		secretsStore:  []cache.Store{},
	}
	informersSynced := []cache.InformerSynced{}

	for _, c := range k8sClients {
		factory := informers.NewSharedInformerFactory(c.source, time.Minute)

		ingressInformer := getIngressInformer(factory, c.source)
		a.EventsIngresses(ctx, ingressInformer)

		ingressStore := IngressStore{
			Store:                 ingressInformer.GetStore(),
			Maintenance:           c.maintenance,
			KubernetesClusterName: c.kubernetesClusterName,
		}

		a.ingressStores = append(a.ingressStores, ingressStore)

		a.factories = append(a.factories, &factory)
		informersSynced = append(informersSynced, ingressInformer.HasSynced)

		if syncSecrets {
			tlsFilter := informers.WithTweakListOptions(func(lo *metav1.ListOptions) {
				lo.FieldSelector = "type=kubernetes.io/tls"
			})
			// using new factory here to apply filter to secrets lister only
			// see https://github.com/kubernetes/kubernetes/issues/90262#issuecomment-671479190
			secretsFactory := informers.NewSharedInformerFactoryWithOptions(c.source, time.Minute, tlsFilter)
			secretsInformer := secretsFactory.Core().V1().Secrets().Informer()
			a.EventsSecrets(ctx, secretsInformer)
			a.secretsStore = append(a.secretsStore, secretsInformer.GetStore())
			informersSynced = append(informersSynced, secretsInformer.HasSynced)
		}
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
