package k8s

// import (
// 	"context"
// 	"testing"
// 	"time"

// 	"k8s.io/apimachinery/pkg/watch"
// 	"k8s.io/client-go/informers"
// 	"k8s.io/client-go/kubernetes/fake"
// 	"k8s.io/client-go/tools/cache"
// )

// // func TestListReturnsEmptyWithNoObjects(t *testing.T) {
// // 	ctx, cancel := context.WithCancel(context.Background())
// // 	defer cancel()
// // 	a := Aggregator{
// // 		events:        make(chan SyncDataEvent, watch.DefaultChanSize),
// // 		ingressStores: []cache.Store{},
// // 	}

// // 	client1 := fake.NewSimpleClientset()

// // 	for _, c := range []*fake.Clientset{client1} {
// // 		factory := informers.NewSharedInformerFactory(c, time.Minute)

// // 		ingressInformer := factory.Extensions().V1beta1().Ingresses().Informer()
// // 		a.EventsIngresses(ctx, ingressInformer)
// // 		a.ingressStores = append(a.ingressStores, ingressInformer.GetStore())

// // 		a.factories = append(a.factories, &factory)
// // 	}

// // 	ingresses, _ := a.ListIngresses()
// // 	if len(ingresses) != 0 {
// // 		t.Errorf("expected 0 ingresses, was %d", len(ingresses))
// // 	}
// // }
