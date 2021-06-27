package controllers

import (
	"context"
	mountv1 "github.com/juicedata/juicefs-csi-driver/pkg/apis/juicefs.com/v1"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("test-juice-mount-reconcile", func() {
	var (
		mount *mountv1.JuiceMount
	)

	BeforeEach(func() {
		mount = &mountv1.JuiceMount{
			TypeMeta: metav1.TypeMeta{
				Kind:       mountv1.Kind,
				APIVersion: mountv1.Version,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-mount",
				Namespace: "default",
			},
			Spec: mountv1.JuiceMountSpec{
				MountSpec: mountv1.MountSpec{
					Image:       "juicedata/juicefs-csi-driver:v0.7.0",
					MetaUrl:     "redis://172.20.10.12:6379/0",
					JuiceFsPath: "/bin/mount.juicefs",
					MountPath:   "/var/run/juice",
				},
				NodeName: "minikube",
			},
		}
	})

	Context("test-mount-health", func() {
		It("mount-running", func() {
			By("apply new juice mount")
			err := k8sClient.Create(context.TODO(), mount)
			Expect(err).ToNot(HaveOccurred())

			created := &mountv1.JuiceMount{}
			timeout := 60
			interval := 5

			By("Wait mount success.")
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: "default",
					Name:      "test-mount",
				}, created); err != nil {
					return false
				}
				return created.Status.MountStatus == mountv1.JMountSuccess
			}, timeout, interval).Should(BeTrue())

			pods := &corev1.PodList{}
			err = k8sClient.List(context.Background(), pods,
				client.InNamespace(mount.Namespace),
				client.MatchingLabels{mountv1.PodMountRef: mount.Name})
			Expect(err).ToNot(HaveOccurred())

			for _, pod := range pods.Items {
				Expect(pod.Status.Phase).Should(Equal(corev1.PodRunning))
				for _, cn := range pod.Status.ContainerStatuses {
					Expect(cn.State.Running).ShouldNot(BeNil())
				}
			}

			Expect(created.Status.MountStatus == mountv1.JMountSuccess).Should(BeTrue())
		})

		AfterEach(func() {
			By("clean up juice mount")
			Expect(k8sClient.Delete(context.TODO(), mount)).Should(Succeed())

			timeout := 60
			interval := 5
			Eventually(func() bool {
				pods := &corev1.PodList{}
				if err := k8sClient.List(context.Background(), pods,
					client.InNamespace(mount.Namespace),
					client.MatchingLabels{mountv1.PodMountRef: mount.Name}); err != nil && apierrors.IsNotFound(err) {
					return true
				} else if err != nil {
					return false
				}
				return len(pods.Items) == 0
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("test-mount-pod-delete", func() {
		It("mount-running", func() {
			By("apply new juice mount")
			err := k8sClient.Create(context.TODO(), mount)
			Expect(err).ToNot(HaveOccurred())

			created := &mountv1.JuiceMount{}
			timeout := 60
			interval := 5

			By("Wait mount success.")
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: "default",
					Name:      "test-mount",
				}, created); err != nil {
					return false
				}
				return created.Status.MountStatus == mountv1.JMountSuccess
			}, timeout, interval).Should(BeTrue())

			pods := &corev1.PodList{}
			err = k8sClient.List(context.Background(), pods,
				client.InNamespace(mount.Namespace),
				client.MatchingLabels{mountv1.PodMountRef: mount.Name})
			Expect(err).ToNot(HaveOccurred())

			Expect(len(pods.Items)).Should(Equal(1))
			Expect(util.IsPodReady(&pods.Items[0])).Should(BeTrue())

			By("delete pod")
			err = k8sClient.Delete(context.TODO(), &pods.Items[0])
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: "default",
					Name:      "test-mount",
				}, created); err != nil {
					return false
				}
				return created.Status.MountStatus == mountv1.JMountFailed
			}, timeout, interval).Should(BeTrue())
		})

		AfterEach(func() {
			By("clean up juice mount")
			Expect(k8sClient.Delete(context.TODO(), mount)).Should(Succeed())

			timeout := 60
			interval := 5
			Eventually(func() bool {
				pods := &corev1.PodList{}
				if err := k8sClient.List(context.Background(), pods,
					client.InNamespace(mount.Namespace),
					client.MatchingLabels{mountv1.PodMountRef: mount.Name}); err != nil && apierrors.IsNotFound(err) {
					return true
				} else if err != nil {
					return false
				}
				return len(pods.Items) == 0
			}, timeout, interval).Should(BeTrue())
		})
	})
})
