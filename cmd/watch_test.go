package cmd

import (
	"testing"

	eventsPort "github.com/ebuildy/kubectl-notify/internal/port/datasources/events"
)

// TestBuildFilter covers the four flag combinations: namespace-only,
// labels-only, both, and neither.
func TestBuildFilter(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		labels    string
		want      eventsPort.Filter
	}{
		{
			name:      "namespace only",
			namespace: "kube-system",
			labels:    "",
			want:      eventsPort.Filter{"namespace": "kube-system"},
		},
		{
			name:      "labels only",
			namespace: "",
			labels:    "app=nginx",
			want:      eventsPort.Filter{"labels": "app=nginx"},
		},
		{
			name:      "both",
			namespace: "default",
			labels:    "app=nginx",
			want:      eventsPort.Filter{"namespace": "default", "labels": "app=nginx"},
		},
		{
			name:      "neither",
			namespace: "",
			labels:    "",
			want:      eventsPort.Filter{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFilter(tt.namespace, tt.labels)
			if len(got) != len(tt.want) {
				t.Fatalf("filter = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("filter[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
