/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	mountv1 "github.com/juicedata/juicefs-csi-driver/pkg/apis/juicefs.com/v1"
	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	reconciler "github.com/juicedata/juicefs-csi-driver/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	_ "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var controllerLog = ctrl.Log.WithName("juicefs-operator")

// JuicefsReconciler reconciles a Juicefs object
type JuicefsReconciler struct {
	Client
	Scheme *runtime.Scheme
}

type Client struct {
	client.Client
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=mount.juicefs.com,resources=juicemount,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mount.juicefs.com,resources=juicemount/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mount.juicefs.com,resources=juicemount/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Modify the Reconcile function to compare the state specified by
// the JuiceMount object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (j *JuicefsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	controllerLog.Info("Receive event.", "name", req.Name, "namespace", req.Namespace)

	result := common.NewResult(ctx)
	// fetching jfsMount instance
	jfsMountInstance := &mountv1.JuiceMount{}
	requeue, err := j.fetchJuiceMount(ctx, req.NamespacedName, jfsMountInstance)
	if err != nil || requeue {
		return ctrl.Result{}, err
	}

	// todo CR check

	sts := reconciler.NewStatus(jfsMountInstance)
	internalResult := j.internalReconcile(ctx, jfsMountInstance, sts)
	err = j.updateStatus(ctx, jfsMountInstance, sts)
	if err != nil {
		controllerLog.Error(err, "err happen when update juice mount status")
		result = result.WithError(fmt.Errorf("update status faoled: %s", err.Error()))
	}
	if err != nil {
		if apierrors.IsConflict(err) {
			controllerLog.Info("Conflict while updating status", "namespace", jfsMountInstance.Namespace, "name", jfsMountInstance.Name)
			return reconcile.Result{Requeue: true}, nil
		}
	}
	return result.WithResult(internalResult).Aggregate()
}

func (j *JuicefsReconciler) fetchJuiceMount(ctx context.Context, name types.NamespacedName, juiceMount *mountv1.JuiceMount) (bool, error) {
	if err := j.Get(ctx, name, juiceMount); err != nil && apierrors.IsNotFound(err) {
		controllerLog.Info("juicemount is deleted. Delete all child resources", "name", name.Name, "namespace", name.Namespace)
		return true, j.onDelete(name)
	} else if err != nil {
		controllerLog.Error(err, "get juice mount cr failed", "namespace", name.Namespace, "name", name.Name)
		return true, err
	}
	return false, nil
}

// internal check
func (j *JuicefsReconciler) internalReconcile(ctx context.Context, juiceMount *mountv1.JuiceMount, status *reconciler.Status) *common.Results {
	if status.Status.MountStatus == "" {
		// need init first
		controllerLog.Info("juicefs mount need init.")
		return j.InitJuiceMount(ctx, status)
	}
	results := common.NewResult(ctx)

	//deal with jm is deleted
	if juiceMount.IsMarkDeleted() {
		controllerLog.Info("juicemount %v is marked deleted.", juiceMount.Name)
		return results.WithError(j.onDelete(types.NamespacedName{
			Namespace: juiceMount.Namespace,
			Name:      juiceMount.Name,
		}))
	}

	// todo mount check

	resourceParam := reconciler.ResourceParameters{
		JM:       *juiceMount,
		Client:   j.Client,
		Recorder: j.Recorder,
	}
	resourceReconciler := reconciler.NewResourceReconciler(resourceParam)
	resourceResult := resourceReconciler.Reconcile(ctx, status)
	return results.WithResult(resourceResult)
}

func (j *JuicefsReconciler) updateStatus(ctx context.Context, juiceMount *mountv1.JuiceMount, status *reconciler.Status) error {
	events, crt := status.Apply()
	if crt == nil {
		return nil
	}

	// record event to k8s
	for _, evt := range events {
		controllerLog.Info("Record events", "event", evt)
		j.Recorder.Event(juiceMount, evt.EventType, evt.Reason, evt.Message)
	}

	// update status to k8s
	controllerLog.Info("Update juiceMount status", "namespace", crt.Namespace, "name", crt.Name)
	return j.Client.Status().Update(ctx, crt)
}

func (j *JuicefsReconciler) onDelete(owner types.NamespacedName) error {
	return reconciler.GarbageCollectSoftOwnedResource(j.Client, owner)
}

// SetupWithManager sets up the controller with the Manager.
func (j *JuicefsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mountv1.JuiceMount{}).
		Owns(&corev1.Pod{}).Complete(j)
}

func (j *JuicefsReconciler) InitJuiceMount(ctx context.Context, status *reconciler.Status) *common.Results {
	return common.NewResult(ctx).With("init-juice-mount", func() (reconcile.Result, error) {
		status.Status.MountStatus = mountv1.JMountInit
		controllerLog.Info("init new mount")
		status.Events = []common.Event{{
			EventType: corev1.EventTypeNormal,
			Reason:    "Init",
			Message:   "init new mount",
		}}
		return reconcile.Result{Requeue: true}, nil
	})
}
