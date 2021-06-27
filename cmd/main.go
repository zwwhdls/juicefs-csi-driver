/*
Copyright 2018 The Kubernetes Authors.

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

package main

import (
	"flag"
	"fmt"
	"github.com/juicedata/juicefs-csi-driver/cmd/apps"
	"k8s.io/klog"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/juicedata/juicefs-csi-driver/pkg/driver"
	"github.com/spf13/cobra"
)

var setupLog = ctrl.Log.WithName("setup")

var (
	endpoint string
	version  bool
	nodeID   string
)

func JuiceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "juicefs",
		Short: "run juice csi driver",
		Run: func(cmd *cobra.Command, args []string) {
			setupLog.Info("juicefs command begin.")
			if version {
				info, err := driver.GetVersionJSON()
				if err != nil {
					setupLog.Error(err, "setup err.")
				}
				fmt.Println(info)
				os.Exit(0)
			}

			if nodeID == "" {
				setupLog.Error(nil, "nodeID must be provided")
			}

			drv, err := driver.NewDriver(endpoint, nodeID)
			if err != nil {
				setupLog.Error(err, "driver init err.")
			}
			if err := drv.Run(); err != nil {
				setupLog.Error(err, "driver run err.")
			}
		},
	}
	cmd.Flags().StringVar(&endpoint, "endpoint", "unix://tmp/csi.sock", "CSI Endpoint")
	cmd.Flags().BoolVar(&version, "version", false, "Print the version and exit.")
	cmd.Flags().StringVar(&nodeID, "nodeid", "", "Node ID")

	fs := flag.NewFlagSet("", flag.PanicOnError)
	klog.InitFlags(fs)
	cmd.Flags().AddGoFlagSet(fs)
	return cmd
}

func main() {
	rootCmd := &cobra.Command{
		Use:          "",
		Short:        "JuiceFS csi driver",
		SilenceUsage: true,
	}
	rootCmd.AddCommand(JuiceCommand())
	rootCmd.AddCommand(apps.Command())

	_ = rootCmd.Execute()
}
