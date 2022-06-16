package k8s

import (
	"context"

	"github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
)

func (a *Aggregator) EventsIngresses(ctx context.Context, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data, ok := obj.(*v1beta1.Ingress)
				if !ok {
					logrus.Warnf("Invalid data from k8s api, %s", obj)
					return
				}
				a.events <- SyncDataEvent{SyncType: INGRESS, Data: data}
				logrus.Debugf("adding %+v", obj)
			},
			DeleteFunc: func(obj interface{}) {
				data, ok := obj.(*v1beta1.Ingress)
				if !ok {
					logrus.Warnf("Invalid data from k8s api, %s", obj)
					return
				}
				a.events <- SyncDataEvent{SyncType: INGRESS, Data: data}
				logrus.Debugf("deleting %+v", obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				dataOld, ok := oldObj.(*v1beta1.Ingress)
				if !ok {
					logrus.Warnf("Invalid data from k8s api, %s", dataOld)
					return
				}
				dataNew, ok := newObj.(*v1beta1.Ingress)
				if !ok {
					logrus.Warnf("Invalid data from k8s api, %s", dataNew)
					return
				}
				a.events <- SyncDataEvent{SyncType: INGRESS, Data: dataNew}
				logrus.Debugf("updating %+v", newObj)
			},
		},
	)
	go informer.Run(ctx.Done())
}
