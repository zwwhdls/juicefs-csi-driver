package reconcile

import (
	"context"
	"fmt"
	mountv1 "github.com/juicedata/juicefs-csi-driver/pkg/apis/juicefs.com/v1"
	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var podLog = ctrl.Log.WithName("pod-reconciler")

type PodDriver struct {
	Client   client.Client
	handlers map[podStatus]podHandler
}

func NewPodDriver(client client.Client) *PodDriver {
	return &PodDriver{
		Client: client,
		handlers: map[podStatus]podHandler{
			podReady:   podReadyHandler,
			podRunning: podRunningHandler,
			podError:   podErrorHandler,
		},
	}
}

type podHandler func(ctx context.Context, pod *corev1.Pod, reconcileStatus *Status) *common.Results
type podStatus string

const (
	podInit    podStatus = "podInit"
	podReady   podStatus = "podReady"
	podError   podStatus = "podError"
	podRunning podStatus = "podRunning"
)

func (p *PodDriver) Run(ctx context.Context, juiceMount mountv1.JuiceMount, current *corev1.Pod, reconcileStatus *Status) *common.Results {
	expected := newMountPod(juiceMount)
	name := expected.GetName()
	result := common.NewResult(ctx)

	if reconcileStatus.Status.MountStatus == mountv1.JMountInit && current == nil {
		podLog.Info("Resource not exist, create it.", "pod", name)
		e := p.Client.Create(context.Background(), expected)
		if e != nil {
			return result.With("PodCreate", func() (reconcile.Result, error) {
				return reconcile.Result{
					Requeue: true,
				}, e
			})
		}
		reconcileStatus.Status.MountStatus = mountv1.JMountRunning
		reconcileStatus.Events = append(reconcileStatus.Events, common.Event{
			EventType: corev1.EventTypeNormal,
			Reason:    "Created",
			Message:   fmt.Sprintf("pod of %s is created, pod name: %s", juiceMount.Name, expected.Name),
		})
		return result.With("mount-pod-create", func() (reconcile.Result, error) {
			return reconcile.Result{Requeue: true}, nil
		})
	}
	return result.WithResult(p.handlers[p.getPodStatus(current)](ctx, current, reconcileStatus))
}

func (p *PodDriver) getPodStatus(pod *corev1.Pod) podStatus {
	if pod == nil || pod.DeletionTimestamp != nil || pod.Status.Phase == corev1.PodUnknown {
		return podError
	}
	if util.IsPodReady(pod) {
		return podReady
	}
	return podRunning
}

func podErrorHandler(ctx context.Context, pod *corev1.Pod, reconcileStatus *Status) *common.Results {
	result := common.NewResult(ctx)
	return result.With("mount-pod-error", func() (reconcile.Result, error) {
		reconcileStatus.Events = append(reconcileStatus.Events, common.Event{
			EventType: corev1.EventTypeWarning,
			Reason:    "Error",
			Message:   fmt.Sprintf("pod %s is error", pod.Name),
		})
		reconcileStatus.Status.MountStatus = mountv1.JMountFailed
		return reconcile.Result{}, nil
	})
}

func podReadyHandler(ctx context.Context, pod *corev1.Pod, reconcileStatus *Status) *common.Results {
	result := common.NewResult(ctx)
	return result.With("mount-pod-ready", func() (reconcile.Result, error) {
		reconcileStatus.Events = append(reconcileStatus.Events, common.Event{
			EventType: corev1.EventTypeNormal,
			Reason:    "Ready",
			Message:   fmt.Sprintf("pod %s is ready", pod.Name),
		})
		reconcileStatus.Status.MountStatus = mountv1.JMountSuccess
		return reconcile.Result{}, nil
	})
}

func podRunningHandler(ctx context.Context, pod *corev1.Pod, reconcileStatus *Status) *common.Results {
	result := common.NewResult(ctx)
	return result.With("mount-pod-running", func() (reconcile.Result, error) {
		reconcileStatus.Events = append(reconcileStatus.Events, common.Event{
			EventType: corev1.EventTypeNormal,
			Reason:    "Running",
			Message:   fmt.Sprintf("pod %s is running", pod.Name),
		})
		reconcileStatus.Status.MountStatus = mountv1.JMountRunning
		return reconcile.Result{Requeue: true}, nil
	})
}

func newMountPod(instance mountv1.JuiceMount) *corev1.Pod {
	isPrivileged := true
	mp := corev1.MountPropagationBidirectional
	dir := corev1.HostPathDirectory
	var pod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", instance.Name),
			Namespace:    instance.Namespace,
			Labels: map[string]string{
				mountv1.PodMountRef: instance.Name,
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         mountv1.Version,
				Kind:               instance.Kind,
				Name:               instance.Name,
				UID:                instance.UID,
				BlockOwnerDeletion: pointer.BoolPtr(true),
				Controller:         pointer.BoolPtr(true),
			}},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            "jfs-mount",
				Image:           instance.Spec.MountSpec.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{"sh", "-c", fmt.Sprintf("%v %v %v && sleep infinity",
					instance.Spec.MountSpec.JuiceFsPath, instance.Spec.MountSpec.MetaUrl, instance.Spec.MountSpec.MountPath)},
				SecurityContext: &corev1.SecurityContext{
					Privileged: &isPrivileged,
				},
				Resources: parsePodResources(),
				VolumeMounts: []corev1.VolumeMount{{
					Name:             "jfs-dir",
					MountPath:        "/jfs",
					MountPropagation: &mp,
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "jfs-dir",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: juicefs.MountPointPath,
						Type: &dir,
					},
				},
			}},
			NodeName: instance.Spec.NodeName,
		},
	}
	return pod
}

func parsePodResources() corev1.ResourceRequirements {
	podLimit := corev1.ResourceList{}
	podRequest := corev1.ResourceList{}
	if juicefs.MountPodCpuLimit != "" {
		podLimit.Cpu().Add(resource.MustParse(juicefs.MountPodCpuLimit))
	}
	if juicefs.MountPodMemLimit != "" {
		podLimit.Memory().Add(resource.MustParse(juicefs.MountPodMemLimit))
	}
	if juicefs.MountPodCpuRequest != "" {
		podRequest.Cpu().Add(resource.MustParse(juicefs.MountPodCpuRequest))
	}
	if juicefs.MountPodMemRequest != "" {
		podRequest.Memory().Add(resource.MustParse(juicefs.MountPodMemRequest))
	}
	return corev1.ResourceRequirements{
		Limits:   podLimit,
		Requests: podRequest,
	}
}
