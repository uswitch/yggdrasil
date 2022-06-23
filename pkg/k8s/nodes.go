package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

type IngressNodeSelector map[string]string
type IngressEndpoints []string

func ClusterLoadbalancersIp(Nodeselector IngressNodeSelector, Endpoints IngressEndpoints, clientSet *kubernetes.Clientset) IngressEndpoints {
	ips := IngressEndpoints{}
	if len(Endpoints) > 0 {
		ips := Endpoints
		return ips
	} else {

		for key, value := range Nodeselector {
			labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{key: value}}
			listOptions := metav1.ListOptions{
				LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
				Limit:         100,
			}
			nodes, err := clientSet.CoreV1().Nodes().List(context.TODO(), listOptions)
			if err != nil {
				panic(err)
			}
			for i := 0; i < len(nodes.Items); i++ {
				nodeip := []corev1.NodeAddress{}
				nodeip = nodes.Items[i].Status.Addresses
				ips = append(ips, nodeip[0].Address)
			}
		}
		return ips
	}
}
