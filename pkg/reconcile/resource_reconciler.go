package reconcile

import (
	"context"
	mountv1 "github.com/juicedata/juicefs-csi-driver/pkg/apis/juicefs.com/v1"
	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var log = ctrl.Log.WithName("resource-reconciler")

func NewResourceReconciler(parameters ResourceParameters) *ResourceReconciler {
	return &ResourceReconciler{ResourceParameters: parameters}
}

type ResourceReconciler struct {
	ResourceParameters
}

type ResourceParameters struct {
	JM mountv1.JuiceMount

	Client   client.Client
	Recorder record.EventRecorder
}

func (d *ResourceReconciler) Reconcile(ctx context.Context, reconcileState *Status) *common.Results {
	results := common.NewResult(ctx)

	// pods
	pod, err := d.fetchResourceFromK8sApi()
	if err != nil {
		log.Error(err, "fetch res failed")
		return results.WithError(err)
	}
	podDriver := NewPodDriver(d.Client)
	podResult := podDriver.Run(ctx, d.JM, pod, reconcileState)
	return results.WithResult(podResult)
}

func GarbageCollectSoftOwnedResource(c client.Client, owner types.NamespacedName) error {
	pods := &corev1.PodList{}

	err := c.List(context.Background(), pods,
		client.InNamespace(owner.Namespace),
		client.MatchingLabels{mountv1.PodMountRef: owner.Name})
	if err != nil {
		log.Error(err, "Select pods error",
			"labels", map[string]string{mountv1.PodMountRef: owner.Name})
		return err
	}
	for _, pod := range pods.Items {
		err = c.Delete(context.Background(), &pod)
		if err != nil {
			log.Error(err, "Pod delete error",
				"namespace", owner.Namespace, "name", pod.Name)
			return err
		}
	}
	return nil
}

func (d *ResourceReconciler) fetchResourceFromK8sApi() (*corev1.Pod, error) {
	pods := &corev1.PodList{}

	err := d.Client.List(context.Background(), pods,
		client.InNamespace(d.JM.Namespace),
		client.MatchingLabels{mountv1.PodMountRef: d.JM.Name})
	if err != nil {
		log.Error(err, "Select pods error", map[string]string{mountv1.PodMountRef: d.JM.Name})
		return nil, err
	}
	if len(pods.Items) != 0 {
		return &pods.Items[0], nil
	}
	return nil, nil
}
