package client

import (
	"context"
	"fmt"
	mountv1 "github.com/juicedata/juicefs-csi-driver/pkg/apis/juicefs.com/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"reflect"
	"testing"
)

func TestNewForConfig(t *testing.T) {
	tests := []struct {
		name    string
		want    *JuiceFsClient
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "aa",
			want: &JuiceFsClient{ },
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewForConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewForConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewForConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJuiceFsClient_JuiceMounts(t *testing.T) {
	type args struct {
		namespace string
		created   *mountv1.JuiceMount
	}
	tests := []struct {
		name   string
		args   args
		want   JuiceMountInterface
	}{
		// TODO: Add test cases.
		{
			name:   "aa",
			args:   args{
				namespace: "kube-system",
				created:   newJuiceMount("redis://192.168.50.53/0", "/data"),
			},
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j, _ := NewForConfig()
			if got, err := j.JuiceMounts(tt.args.namespace).Create(context.Background(), tt.args.created); !reflect.DeepEqual(got, tt.want) {
				klog.V(5).Info(err)
				t.Errorf("JuiceMounts() = %v, want %v", got, tt.want)
			}
		})
	}
}
func newJuiceMount(source, mountPath string) *mountv1.JuiceMount{
	return &mountv1.JuiceMount{
		TypeMeta: metav1.TypeMeta{
			Kind:       mountv1.Kind,
			APIVersion: fmt.Sprintf("%s/%s", mountv1.Group, mountv1.Version),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "kube-system",
		},
		Spec: mountv1.JuiceMountSpec{
			MountSpec: mountv1.MountSpec{
				Image:       "nginx",
				MetaUrl:     source,
				JuiceFsPath: "/bin",
				MountPath:   mountPath,
			},
			NodeName: "minikube",
		},
	}
}
