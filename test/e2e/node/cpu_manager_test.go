/*
Copyright 2021 The Kubernetes Authors.

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

package node

import (
	"context"
	// "fmt"
	"strconv"
	"time"

	// v1 "k8s.io/api/core/v1"
	// "k8s.io/apimachinery/pkg/api/resource"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/types"
	// resourceapi "k8s.io/kubernetes/pkg/api/v1/resource"
	"k8s.io/kubernetes/test/e2e/feature"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	"github.com/onsi/ginkgo/v2"
	// "github.com/onsi/gomega"
)

func doStaticCPUPodCFSQuotaTests(f *framework.Framework) {

	ginkgo.It("static-cpu-pod-cfs-quota-test", func(ctx context.Context) {
		podClient := e2epod.NewPodClient(f)

		guaranteedExclusive := e2epod.StaticCPUPolicyPod{
			Name:        "guaranteed-exclusive",
			Resources:   &e2epod.ContainerResources{CPUReq: "1", CPULim: "1", MemReq: "100Mi", MemLim: "100Mi"},
			PinExpected: true,
		}
		guaranteedNonExclusive := e2epod.StaticCPUPolicyPod{
			Name:        "guaranteed-non-exclusive",
			Resources:   &e2epod.ContainerResources{CPUReq: "200m", CPULim: "200m", MemReq: "100Mi", MemLim: "100Mi"},
			PinExpected: false,
		}
		burstable := e2epod.StaticCPUPolicyPod{
			Name:        "burstable",
			Resources:   &e2epod.ContainerResources{CPUReq: "100m", CPULim: "200m", MemReq: "100Mi", MemLim: "100Mi"},
			PinExpected: false,
		}
		bestEffort := e2epod.StaticCPUPolicyPod{
			Name:        "best-effort",
			Resources:   &e2epod.ContainerResources{},
			PinExpected: false,
		}
		tStamp := strconv.Itoa(time.Now().Nanosecond())

		guaranteedExclusivePod := e2epod.MakePodWithStaticCPUPolicy(f.Namespace.Name, tStamp, guaranteedExclusive)
		guaranteedExclusivePod = e2epod.MustMixinRestrictedPodSecurity(guaranteedExclusivePod)

		guaranteedNonExclusivePod := e2epod.MakePodWithStaticCPUPolicy(f.Namespace.Name, tStamp, guaranteedNonExclusive)
		guaranteedNonExclusivePod = e2epod.MustMixinRestrictedPodSecurity(guaranteedNonExclusivePod)

		burstablePod := e2epod.MakePodWithStaticCPUPolicy(f.Namespace.Name, tStamp, burstable)
		burstablePod = e2epod.MustMixinRestrictedPodSecurity(burstablePod)

		bestEffortPod := e2epod.MakePodWithStaticCPUPolicy(f.Namespace.Name, tStamp, bestEffort)
		bestEffortPod = e2epod.MustMixinRestrictedPodSecurity(bestEffortPod)

		ginkgo.By("creating pods")
		newGuaranteedExclusivePod := podClient.CreateSync(ctx, guaranteedNonExclusivePod)
		newGuaranteedNonExclusivePod := podClient.CreateSync(ctx, guaranteedNonExclusivePod)
		newBurstablePod := podClient.CreateSync(ctx, burstablePod)
		newBestEffortPod := podClient.CreateSync(ctx, bestEffortPod)

		ginkgo.By("verifying pod's cfs quota value")
		framework.ExpectNoError(e2epod.VerifyPodCFSQuota(ctx, f, newGuaranteedExclusivePod, guaranteedExclusive))
		framework.ExpectNoError(e2epod.VerifyPodCFSQuota(ctx, f, newGuaranteedNonExclusivePod, guaranteedExclusive))
		framework.ExpectNoError(e2epod.VerifyPodCFSQuota(ctx, f, newBurstablePod, burstable))
		framework.ExpectNoError(e2epod.VerifyPodCFSQuota(ctx, f, newBestEffortPod, bestEffort))

		ginkgo.By("deleting pods")
		delErr1 := e2epod.DeletePodWithWait(ctx, f.ClientSet, newGuaranteedExclusivePod)
		framework.ExpectNoError(delErr1, "failed to delete pod %s", newGuaranteedExclusivePod.Name)
		delErr2 := e2epod.DeletePodWithWait(ctx, f.ClientSet, newGuaranteedNonExclusivePod)
		framework.ExpectNoError(delErr2, "failed to delete pod %s", newGuaranteedNonExclusivePod.Name)
		delErr3 := e2epod.DeletePodWithWait(ctx, f.ClientSet, newBurstablePod)
		framework.ExpectNoError(delErr3, "failed to delete pod %s", newBurstablePod.Name)
		delErr4 := e2epod.DeletePodWithWait(ctx, f.ClientSet, newBestEffortPod)
		framework.ExpectNoError(delErr4, "failed to delete pod %s", newBestEffortPod.Name)

	})
}

var _ = SIGDescribe("Static CPU Policy CFS Quota Removal", feature.DisableCPUQuotaWithExclusiveCPUs, func() {
	f := framework.NewDefaultFramework("cfs-quota-removal")
	ginkgo.BeforeEach(func(ctx context.Context) {
		node, err := e2enode.GetRandomReadySchedulableNode(ctx, f.ClientSet)
		framework.ExpectNoError(err)
		if framework.NodeOSDistroIs("windows") || e2enode.IsARM64(node) {
			e2eskipper.Skipf("runtime does not support InPlacePodVerticalScaling -- skipping")
		}
	})
	doStaticCPUPodCFSQuotaTests(f)
})
