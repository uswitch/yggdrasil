package envoy

import (
	"testing"

	"k8s.io/api/extensions/v1beta1"
)

func TestGenerate(t *testing.T) {
	ingresses := []v1beta1.Ingress{
		newIngress("wibble", "bibble"),
	}

	configurator := NewKubernetesConfigurator("a", Certificate{Cert: "b", Key: "c"}, "d", []string{"e"})

	snapshot := configurator.Generate(ingresses)

	if len(snapshot.Listeners.Items) != 1 {
		t.Fatalf("Num listeners: %d", len(snapshot.Listeners.Items))
	}
}
