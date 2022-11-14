package k8s

// SyncType represents the type of k8s received message
type SyncType string

// SyncDataEvent represents converted k8s received message
type SyncDataEvent struct {
	_ [0]int
	SyncType
	Data interface{}
}

const (
	COMMAND SyncType = "COMMAND"
	INGRESS SyncType = "INGRESS"
)
