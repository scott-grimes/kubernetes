/*
Copyright 2024 The Kubernetes Authors.

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

package pod

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubecm "k8s.io/kubernetes/pkg/kubelet/cm"
	"k8s.io/kubernetes/test/e2e/framework"
	imageutils "k8s.io/kubernetes/test/utils/image"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

type StaticCPUPolicyPod struct {
	Name        string
	Resources   *ContainerResources
	PinExpected bool
}

func getStaticCPUPodTestResources(tcInfo StaticCPUPolicyPod) v1.ResourceRequirements {
	var res v1.ResourceRequirements

	if tcInfo.Resources != nil {
		var lim, req v1.ResourceList
		if tcInfo.Resources.CPULim != "" || tcInfo.Resources.MemLim != "" || tcInfo.Resources.EphStorLim != "" {
			lim = make(v1.ResourceList)
		}
		if tcInfo.Resources.CPUReq != "" || tcInfo.Resources.MemReq != "" || tcInfo.Resources.EphStorReq != "" {
			req = make(v1.ResourceList)
		}
		if tcInfo.Resources.CPULim != "" {
			lim[v1.ResourceCPU] = resource.MustParse(tcInfo.Resources.CPULim)
		}
		if tcInfo.Resources.MemLim != "" {
			lim[v1.ResourceMemory] = resource.MustParse(tcInfo.Resources.MemLim)
		}
		if tcInfo.Resources.CPUReq != "" {
			req[v1.ResourceCPU] = resource.MustParse(tcInfo.Resources.CPUReq)
		}
		if tcInfo.Resources.MemReq != "" {
			req[v1.ResourceMemory] = resource.MustParse(tcInfo.Resources.MemReq)
		}
		res = v1.ResourceRequirements{Limits: lim, Requests: req}
	}
	return res
}

func MakePodWithStaticCPUPolicy(ns, timeStamp string, tcInfo StaticCPUPolicyPod) *v1.Pod {

	cmd := "grep Cpus_allowed_list /proc/self/status | cut -f2 && sleep 1d"

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tcInfo.Name,
			Namespace: ns,
			Labels: map[string]string{
				"time": timeStamp,
			},
		},
		Spec: v1.PodSpec{
			OS: &v1.PodOS{Name: v1.Linux},
			Containers: []v1.Container{
				{Name: tcInfo.Name,
					Image:     imageutils.GetE2EImage(imageutils.BusyBox),
					Command:   []string{"/bin/sh"},
					Args:      []string{"-c", cmd},
					Resources: getStaticCPUPodTestResources(tcInfo), // swap to other thing
				},
			},
		},
	}
	return pod
}

func VerifyPodCFSQuota(ctx context.Context, f *framework.Framework, pod *v1.Pod, tcInfo StaticCPUPolicyPod) error { //where if anywhere can i find node policy?
	ginkgo.GinkgoHelper()
	if podOnCgroupv2Node == nil {
		value := isPodOnCgroupv2Node(f, pod)
		podOnCgroupv2Node = &value
	}
	cgroupCPULimit := Cgroupv2CPULimit
	if !*podOnCgroupv2Node {
		cgroupCPULimit = CgroupCPUQuota
	}
	verifyCgroupValue := func(cName, cgPath, expectedCgValue string) error {
		cmd := fmt.Sprintf("head -n 1 %s", cgPath)
		framework.Logf("Namespace %s Pod %s Container %s - looking for cgroup value %s in path %s",
			pod.Namespace, pod.Name, cName, expectedCgValue, cgPath)
		cgValue, _, err := ExecCommandInContainerWithFullOutput(f, pod.Name, cName, "/bin/sh", "-c", cmd)
		if err != nil {
			return fmt.Errorf("failed to find expected value %q in container cgroup %q", expectedCgValue, cgPath)
		}
		cgValue = strings.Trim(cgValue, "\n")
		if cgValue != expectedCgValue {
			return fmt.Errorf("cgroup value %q not equal to expected %q", cgValue, expectedCgValue)
		}
		return nil
	}

	podContainer := pod.Spec.Containers[0]

	if podContainer.Resources.Limits != nil || podContainer.Resources.Requests != nil {
		var expectedCPULimitString string
		cpuLimit := podContainer.Resources.Limits.Cpu()
		cpuQuota := kubecm.MilliCPUToQuota(cpuLimit.MilliValue(), kubecm.QuotaPeriod)
		if cpuLimit.IsZero() || tcInfo.PinExpected {
			cpuQuota = -1
		}
		expectedCPULimitString = strconv.FormatInt(cpuQuota, 10)
		if *podOnCgroupv2Node {
			if expectedCPULimitString == "-1" {
				expectedCPULimitString = "max"
			}
		}
		err := verifyCgroupValue(tcInfo.Name, cgroupCPULimit, expectedCPULimitString)
		if err != nil {
			return err
		}
	}
	return nil
}

func WaitForStaticCPUPod(ctx context.Context, f *framework.Framework, pod *v1.Pod, expected StaticCPUPolicyPod) *v1.Pod {
	ginkgo.GinkgoHelper()
	timeouts := framework.NewTimeoutContext()
	// Verify Pod Containers Cgroup Values
	gomega.Eventually(ctx, VerifyPodCFSQuota, timeouts.PodStartShort, timeouts.Poll).
		WithArguments(f, pod, expected).
		ShouldNot(gomega.HaveOccurred(), "failed to verify container cgroup values to match expected")
	return pod
}
