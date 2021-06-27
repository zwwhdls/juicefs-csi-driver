package apps

import (
	"flag"
	mountv1 "github.com/juicedata/juicefs-csi-driver/pkg/apis/juicefs.com/v1"
	"github.com/juicedata/juicefs-csi-driver/pkg/controllers"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/spf13/cobra"
	"k8s.io/klog"
	"os"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	//+kubebuilder:scaffold:imports
)

var (
	scheme         = runtime.NewScheme()
	setupLog       = ctrl.Log.WithName("setup")
	controllerName = "juicefs-controller"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(mountv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme

	juicefs.NodeName = os.Getenv("NODE_NAME")
	juicefs.MountImage = os.Getenv("MOUNT_IMAGE")
	juicefs.MountPointPath = os.Getenv("JUICEFS_MOUNT_PATH")
}

type Manager struct {
	ctrl.Manager
}

func NewManager() *Manager {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "4fa6ffa6.juicefs.com", // todo
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.JuicefsReconciler{
		Client: controllers.Client{
			Client:   mgr.GetClient(),
			Recorder: mgr.GetEventRecorderFor(controllerName),
		},
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Juicefs")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	return &Manager{mgr}
}

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manager",
		Short: "Start the Juice Mount on Kubernetes operator",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := NewManager()
			setupLog.Info("starting manager")
			err := mgr.Manager.Start(ctrl.SetupSignalHandler())
			setupLog.Error(err, "problem running manager")
			return err
		},
	}
	fs := flag.NewFlagSet("", flag.PanicOnError)
	klog.InitFlags(fs)
	cmd.Flags().AddGoFlagSet(fs)
	cmd.Flags().StringVar(&juicefs.MountPodCpuRequest, "mountPodCpuRequest", "1", "mount pod cpuRequest")
	cmd.Flags().StringVar(&juicefs.MountPodCpuLimit, "mountPodCpuLimit", "1", "mount pod cpuLimit")
	cmd.Flags().StringVar(&juicefs.MountPodMemRequest, "mountPodMemRequest", "1G", "mount pod memoryRequest")
	cmd.Flags().StringVar(&juicefs.MountPodMemLimit, "mountPodMemLimit", "1G", "mount pod memoryLimit")
	return cmd
}
