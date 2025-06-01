package main

import (
	"context"
	"fmt"
	"os"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/component-base/cli"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

type MyAiPlugin struct {
	handle framework.Handle
}

type NodeNameDistance struct {
	Name     string
	Distance int
}

const (
	Name = "MyAiPlugin"
)

var _ = framework.FilterPlugin(&MyAiPlugin{})
var _ = framework.ScorePlugin(&MyAiPlugin{})

func (p *MyAiPlugin) Name() string {
	return Name
}

func (p *MyAiPlugin) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {

	reqCPU := pod.Spec.Containers[0].Resources.Requests.Cpu().MilliValue()
	reqMem := pod.Spec.Containers[0].Resources.Requests.Memory().Value()
	allocCPU := nodeInfo.Allocatable.MilliCPU
	allocMem := nodeInfo.Allocatable.Memory

	if reqCPU > allocCPU || reqMem > allocMem {
		return framework.NewStatus(framework.Unschedulable, "Nedovoljni resursi")
	}
	return framework.NewStatus(framework.Success, "")
}

func (p *MyAiPlugin) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	nodeInfo, _ := p.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	node := nodeInfo.Node()

	perf := map[string]int64{"low": 1, "medium": 5, "high": 10}
	lat := map[string]int64{"high": 1, "medium": 5, "low": 10}

	ps := perf[node.Labels["performance"]]
	ls := lat[node.Labels["latency"]]

	score := ps*2 + ls
	return score, framework.NewStatus(framework.Success, "")
}

func (p *MyAiPlugin) ScoreExtensions() framework.ScoreExtensions {
	return p
}

func (p *MyAiPlugin) NormalizeScore(_ context.Context, _ *framework.CycleState, pod *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	var highest int64 = 0
	lowest := scores[0].Score

	for _, nodeScore := range scores {
		if nodeScore.Score < lowest {
			lowest = nodeScore.Score
		}
		if nodeScore.Score > highest {
			highest = nodeScore.Score
		}
	}

	if highest == lowest {
		lowest--
	}

	for i, nodeScore := range scores {
		scores[i].Score = (nodeScore.Score - lowest) * framework.MaxNodeScore / (highest - lowest)
	}

	return nil
}

func New(obj runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	return &MyAiPlugin{handle: handle}, nil
}

func main() {
	fmt.Println("Starting MyAiPlugin Scheduler...")

	command := app.NewSchedulerCommand(
		app.WithPlugin(Name, New),
	)

	code := cli.Run(command)

	if code == 1 {
		fmt.Println("Registered plugin successfully.")
	}

	os.Exit(code)
}
