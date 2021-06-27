package client

import (
	"context"
	mountv1 "github.com/juicedata/juicefs-csi-driver/pkg/apis/juicefs.com/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

type JuiceMountInterface interface {
	List(ctx context.Context) (*mountv1.JuiceMountList, error)
	Get(ctx context.Context, name string) (*mountv1.JuiceMount, error)
	Create(ctx context.Context, mount *mountv1.JuiceMount) (*mountv1.JuiceMount, error)
	Patch(ctx context.Context, mount *mountv1.JuiceMountApplyConfiguration) (*mountv1.JuiceMount, error)
	Delete(ctx context.Context, name string) error
}

type JuiceMounts struct {
	restClient rest.Interface
	namespace  string
}

func newJuiceMounts(c *JuiceFsClient, namespace string) *JuiceMounts {
	return &JuiceMounts{
		restClient: c.restClient,
		namespace:  namespace,
	}
}
func (j JuiceMounts) List(ctx context.Context) (*mountv1.JuiceMountList, error) {
	result := &mountv1.JuiceMountList{}
	err := j.restClient.
		Get().
		Namespace(j.namespace).
		Resource("juicemounts").
		Do(ctx).
		Into(result)
	return result, err
}

func (j JuiceMounts) Get(ctx context.Context, name string) (*mountv1.JuiceMount, error) {
	result := &mountv1.JuiceMount{}
	err := j.restClient.Get().
		Namespace(j.namespace).
		Resource("juicemounts").
		Name(name).
		Do(ctx).
		Into(result)
	return result, err
}

func (j JuiceMounts) Create(ctx context.Context, mount *mountv1.JuiceMount) (*mountv1.JuiceMount, error) {
	result := &mountv1.JuiceMount{}
	err := j.restClient.Post().
		Namespace(j.namespace).
		Resource("juicemounts").
		Body(mount).
		Do(ctx).
		Into(result)
	return result, err
}

func (j JuiceMounts) Patch(ctx context.Context, mount *mountv1.JuiceMountApplyConfiguration) (*mountv1.JuiceMount, error) {
	result := &mountv1.JuiceMount{}
	err := j.restClient.Patch(types.ApplyPatchType).
		Namespace(j.namespace).
		Resource("juicemounts").
		Name(*mount.Name).
		Body(mount).
		Do(ctx).
		Into(result)
	return result, err
}

func (j JuiceMounts) Delete(ctx context.Context, name string) error {
	return j.restClient.Delete().
		Namespace(j.namespace).
		Resource("juicemounts").
		Name(name).
		Do(ctx).
		Error()
}
