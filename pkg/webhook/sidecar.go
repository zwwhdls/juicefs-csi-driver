package webhook

import (
	"context"
	"fmt"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"strings"
)

const (
	ceMountPath      = "/bin/mount.juicefs"
	sidecarMountPath = "/var/run/juice"
)

func addSidecar(pod corev1.Pod, target []corev1.Container, basePath string, factory func(po corev1.Pod) []corev1.Container) (patch []patchOperation) {
	var value interface{}
	added := factory(pod)
	first := len(target) == 0

	for _, add := range added {
		value = add
		path := basePath
		if first {
			first = false
			value = []corev1.Container{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

func sidecarFactory(pod corev1.Pod) []corev1.Container {
	result := []corev1.Container{}
	for _, volume := range pod.Spec.Volumes {
		secret, err := getSecret(pod.Namespace, volume, getStorageClassByVolume)
		if err != nil || secret == nil {
			continue
		}

		secretEnv := corev1.EnvVar{
			Name: "metaurl",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "metaurl",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secret.Name,
					},
				},
			},
		}

		mp := corev1.MountPropagationBidirectional
		cn := corev1.Container{
			Name:    "juice-csi-sidecar",
			Image:   SidecarImage,
			Command: []string{"sh", "-c", fmt.Sprintf("%v ${metaurl} %v && sleep infinity", ceMountPath, sidecarMountPath)},
			Env:     []corev1.EnvVar{secretEnv},
			VolumeMounts: []corev1.VolumeMount{{
				Name:             volume.Name,
				MountPath:        sidecarMountPath,
				MountPropagation: &mp,
			}},
			Lifecycle: &corev1.Lifecycle{
				PreStop: &corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{"umount", sidecarMountPath},
					},
				},
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(SidecarCpuLimit),
					corev1.ResourceMemory: resource.MustParse(SidecarMemLimit),
				},
			},
		}
		result = append(result, cn)
	}
	return result
}

func updateContainer(pod corev1.Pod, path string, getStorageClass func(namespace string, volume corev1.Volume) (*v1.StorageClass, error)) (patch []patchOperation) {
	containers := pod.Spec.Containers
	volumes := pod.Spec.Volumes
	volumeMap := make(map[string]corev1.Volume)
	for _, volume := range volumes {
		if s, err := getStorageClass(pod.Namespace, volume); err != nil || s == nil {
			continue
		}
		volumeMap[volume.Name] = volume
	}

	mp := corev1.MountPropagationHostToContainer
	for _, container := range containers {
		volumeMounts := container.VolumeMounts
		for i, volumeMount := range volumeMounts {
			if _, ok := volumeMap[volumeMount.Name]; !ok {
				continue
			}
			volumeMount.MountPropagation = &mp
			volumeMounts[i] = volumeMount
		}
		container.VolumeMounts = volumeMounts
		patch = append(patch, patchOperation{
			Op:    "update",
			Path:  path,
			Value: container,
		})
	}
	return patch
}

func updateAnnotation(target map[string]string, added map[string]string) (patch []patchOperation) {
	for key, value := range added {
		if target == nil || target[key] == "" {
			target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{
					key: value,
				},
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/metadata/annotations/" + key,
				Value: value,
			})
		}
	}
	return patch
}

func getClientSet() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func getStorageClassByVolume(namespace string, volume corev1.Volume) (*v1.StorageClass, error) {
	if volume.PersistentVolumeClaim == nil {
		// only if the volume is PVC
		klog.Info("volume is not pvc.")
		return nil, nil
	}
	clientset, err := getClientSet()
	if err != nil {
		klog.Error("get clientset error: %v", err)
		return nil, err
	}
	claimName := volume.PersistentVolumeClaim.ClaimName
	claim, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), claimName, metav1.GetOptions{})
	if err != nil {
		klog.Error("get claim %v in namespace %v error: %v", claimName, namespace, err)
		return nil, err
	}
	storageClass, err := clientset.StorageV1().StorageClasses().Get(context.Background(), *claim.Spec.StorageClassName, metav1.GetOptions{})
	if err != nil {
		klog.Error("get storageClass %v error: %v", claimName, err)
		return nil, err
	}

	if storageClass.Provisioner != driver.DriverName {
		klog.Info("storageClass driver is not juicefs.")
		return nil, nil
	}
	return storageClass, nil
}

func getSecret(namespace string, volume corev1.Volume, getStorageClass func(namespace string, volume corev1.Volume) (*v1.StorageClass, error)) (*corev1.Secret, error) {
	clientset, err := getClientSet()
	if err != nil {
		klog.Error("get clientset error: %v", err)
		return nil, err
	}
	storageClass, err := getStorageClass(namespace, volume)
	if err != nil || storageClass == nil {
		return nil, err
	}

	if storageClass.Parameters == nil || storageClass.Parameters["csi.storage.k8s.io/node-publish-secret-name"] == "" ||
		storageClass.Parameters["csi.storage.k8s.io/node-publish-secret-namespace"] == "" {
		klog.Info("volume %v doesn't have secret.", volume.Name)
		return nil, nil
	}
	klog.Info("secret of juicefs name %v namespace %v", storageClass.Parameters["csi.storage.k8s.io/node-publish-secret-name"],
		storageClass.Parameters["csi.storage.k8s.io/node-publish-secret-namespace"])

	storageSecret, err := clientset.CoreV1().Secrets(storageClass.Parameters["csi.storage.k8s.io/node-publish-secret-namespace"]).
		Get(context.Background(), storageClass.Parameters["csi.storage.k8s.io/node-publish-secret-name"], metav1.GetOptions{})
	if err != nil {
		klog.Error("get storage secret %v error: %v", storageClass.Parameters["csi.storage.k8s.io/node-publish-secret-name"], err)
		return nil, err
	}

	// create secret of storage in pod namespace if not exist
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.Background(), storageSecret.Name, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		namespacedSecret := storageSecret
		source := string(namespacedSecret.Data["metaurl"])
		if !strings.Contains(source, "://") {
			source = "redis://" + source
			namespacedSecret.Data["metaurl"] = []byte(source)
		}
		namespacedSecret.Namespace = namespace
		secret, err = clientset.CoreV1().Secrets(namespace).Create(context.Background(), namespacedSecret, metav1.CreateOptions{})
		if err != nil {
			klog.Error("Can't create secret in namespace: %v", namespace)
			return nil, err
		}
	} else if err != nil {
		klog.Error("Can't get secret in namespace: %v", namespace)
		return nil, err
	}

	return secret, nil
}
