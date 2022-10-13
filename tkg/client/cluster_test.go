// Copyright 2022 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package client_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/api/v1alpha3"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterctl "sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/vmware-tanzu/tanzu-framework/tkg/client"
	"github.com/vmware-tanzu/tanzu-framework/tkg/clusterclient"
	"github.com/vmware-tanzu/tanzu-framework/tkg/constants"
	"github.com/vmware-tanzu/tanzu-framework/tkg/fakes"
	"github.com/vmware-tanzu/tanzu-framework/tkg/region"
	"github.com/vmware-tanzu/tanzu-framework/tkg/tkgconfigbom"
)

var _ = Describe("ValidateManagementClusterVersionWithCLI", func() {
	const (
		clusterName = "test-cluster"
		v140        = "v1.4.0"
		v141        = "v1.4.1"
		v150        = "v1.5.0"
	)
	var (
		regionalClient fakes.ClusterClient
		tkgBomClient   fakes.TKGConfigBomClient
		regionManager  fakes.RegionManager
		c              *TkgClient
		err            error
	)
	JustBeforeEach(func() {
		err = c.ValidateManagementClusterVersionWithCLI(&regionalClient)
	})
	BeforeEach(func() {
		regionManager = fakes.RegionManager{}
		regionManager.GetCurrentContextReturns(region.RegionContext{
			ClusterName: clusterName,
			Status:      region.Success,
		}, nil)

		regionalClient = fakes.ClusterClient{}
		regionalClient.ListResourcesStub = func(i interface{}, lo ...client.ListOption) error {
			list := i.(*v1alpha3.ClusterList)
			*list = v1alpha3.ClusterList{
				Items: []v1alpha3.Cluster{
					{
						ObjectMeta: v1.ObjectMeta{
							Name:      clusterName,
							Namespace: "default",
						},
					},
				},
			}
			return nil
		}

		c, err = New(Options{
			TKGConfigUpdater: &fakes.TKGConfigUpdaterClient{},
			TKGBomClient:     &tkgBomClient,
			RegionManager:    &regionManager,
		})
	})
	Context("v1.4.0 management cluster", func() {
		BeforeEach(func() {
			regionalClient.GetManagementClusterTKGVersionReturns(v140, nil)
		})

		When("management cluster version matches cli version", func() {
			BeforeEach(func() {
				tkgBomClient = fakes.TKGConfigBomClient{}
				tkgBomClient.GetDefaultTKGReleaseVersionReturns(v140, nil)
			})
			It("should validate without error", func() {
				Expect(err).To(BeNil())
			})
		})

		When("cli version is a patch version ahead of management cluster", func() {
			BeforeEach(func() {
				tkgBomClient = fakes.TKGConfigBomClient{}
				tkgBomClient.GetDefaultTKGReleaseVersionReturns(v141, nil)
			})
			It("should validate without error", func() {
				Expect(err).To(BeNil())
			})
		})

		When("cli version is a minor version ahead of management cluster", func() {
			BeforeEach(func() {
				tkgBomClient = fakes.TKGConfigBomClient{}
				tkgBomClient.GetDefaultTKGReleaseVersionReturns(v150, nil)
			})
			It("should return an error", func() {
				Expect(err).Should(MatchError("version mismatch between management cluster and cli version. Please upgrade your management cluster to the latest to continue"))
			})
		})
	})

})

var _ = Describe("CreateCluster", func() {
	const (
		clusterName = "regional-cluster-2"
	)
	var (
		tkgClient                   *TkgClient
		clusterClientFactory        *fakes.ClusterClientFactory
		clusterClient               *fakes.ClusterClient
		featureFlagClient           *fakes.FeatureFlagClient
		tkgBomClient                *fakes.TKGConfigBomClient
		tkgConfigUpdaterClient      *fakes.TKGConfigUpdaterClient
		tkgConfigReaderWriter       *fakes.TKGConfigReaderWriter
		tkgConfigReaderWriterClient *fakes.TKGConfigReaderWriterClient
	)
	BeforeEach(func() {
		clusterClientFactory = &fakes.ClusterClientFactory{}
		clusterClient = &fakes.ClusterClient{}
		clusterClientFactory.NewClientReturns(clusterClient, nil)
		featureFlagClient = &fakes.FeatureFlagClient{}
		tkgBomClient = &fakes.TKGConfigBomClient{}
		tkgConfigUpdaterClient = &fakes.TKGConfigUpdaterClient{}
		tkgConfigReaderWriterClient = &fakes.TKGConfigReaderWriterClient{}
		tkgConfigReaderWriter = &fakes.TKGConfigReaderWriter{}

		tkgConfigReaderWriterClient.TKGConfigReaderWriterReturns(tkgConfigReaderWriter)

		tkgClient, err = CreateTKGClientOptsMutator(configFile2, testingDir, "../fakes/config/bom/tkg-bom-v1.3.1.yaml", 2*time.Second, func(o Options) Options {
			o.ClusterClientFactory = clusterClientFactory
			o.FeatureFlagClient = featureFlagClient
			o.TKGBomClient = tkgBomClient
			o.TKGConfigUpdater = tkgConfigUpdaterClient
			o.ReaderWriterConfigClient = tkgConfigReaderWriterClient
			return o
		})
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Validate controlPlaneTaint in kcp of workload cluster", func() {
		It("Should fail if cluster is single node and kcp has controlPlaneTaint", func() {
			options := CreateClusterOptions{
				ClusterConfigOptions: ClusterConfigOptions{
					KubernetesVersion: "v1.18.0+vmware.2",
					ClusterName:       clusterName,
					TargetNamespace:   constants.DefaultNamespace,
					ProviderRepositorySource: &clusterctl.ProviderRepositorySourceOptions{
						InfrastructureProvider: VSphereProviderName,
					},
					WorkerMachineCount: pointer.Int64Ptr(0),
				},
				IsInputFileClusterClassBased: true,
				ClusterConfigFile:            "../fakes/config/cluster_vsphere.yaml",
			}
			options.Edition = "some edition"
			clusterClient.ListResourcesCalls(func(clusterList interface{}, options ...client.ListOption) error {
				if clusterList, ok := clusterList.(*capiv1alpha3.ClusterList); ok {
					clusterList.Items = []capiv1alpha3.Cluster{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      clusterName,
								Namespace: constants.DefaultNamespace,
							},
						},
					}
					return nil
				}
				return nil
			})

			clusterClient.GetManagementClusterTKGVersionReturns("v1.2.1-rc.1", nil)
			clusterClient.GetRegionalClusterDefaultProviderNameReturns(VSphereProviderName, nil)
			tkgBomClient.GetDefaultTKGReleaseVersionReturns("v1.2.1-rc.1", nil)
			tkgBomClient.GetDefaultTkrBOMConfigurationReturns(&tkgconfigbom.BOMConfiguration{
				Release: &tkgconfigbom.ReleaseInfo{Version: "v1.3"},
				Components: map[string][]*tkgconfigbom.ComponentInfo{
					"kubernetes": {{Version: "v1.18.0+vmware.2"}},
				},
			}, nil)
			tkgBomClient.GetDefaultTkgBOMConfigurationReturns(&tkgconfigbom.BOMConfiguration{
				Release: &tkgconfigbom.ReleaseInfo{Version: "v1.23"},
			}, nil)
			tkgConfigReaderWriter.GetCalls(func(key string) (string, error) {
				if key == constants.ConfigVariableCNI {
					return "antrea", nil
				} else if key == constants.ConfigVariableControlPlaneNodeNameservers || key == constants.ConfigVariableWorkerNodeNameservers {
					return "8.8.8.8", nil
				} else if key == VsphereNodeCPUVarName[0] || key == VsphereNodeCPUVarName[1] {
					return "2", nil
				} else if key == VsphereNodeMemVarName[0] || key == VsphereNodeMemVarName[1] {
					return "4098", nil
				} else if key == VsphereNodeDiskVarName[0] || key == VsphereNodeDiskVarName[1] {
					return "20", nil
				} else if key == constants.ConfigVariableVsphereServer {
					return "10.0.0.1", nil
				} else if key == constants.ConfigVariableWorkerMachineCount0 || key == constants.ConfigVariableWorkerMachineCount1 || key == constants.ConfigVariableWorkerMachineCount2 {
					return "0", nil
				}
				return "192.168.2.1/16", nil
			})

			featureFlagClient.IsConfigFeatureActivatedReturns(true, nil)

			clusterClient.IsPacificRegionalClusterReturns(false, nil)
			clusterClient.GetResourceCalls(func(c interface{}, name, namespace string, pv clusterclient.PostVerifyrFunc, opt *clusterclient.PollOptions) error {
				cc := c.(*capi.Cluster)
				fc := &capi.Cluster{
					Spec: capi.ClusterSpec{
						Topology: &capi.Topology{
							Workers: &capi.WorkersTopology{
								MachineDeployments: []capi.MachineDeploymentTopology{},
							},
							Variables: []capi.ClusterVariable{
								{
									Name: "controlPlaneTaint",
									Value: apiextensionsv1.JSON{
										Raw: []byte("false"),
									},
								},
							},
						},
					},
				}
				*cc = *fc
				return nil
			})

			_, err := tkgClient.CreateCluster(&options, false)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
