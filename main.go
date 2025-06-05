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
	nodeInfo, err := p.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return 0, framework.AsStatus(err)
	}
	node := nodeInfo.Node()

	// Label-based static scores
	perfScoreMap := map[string]int64{"low": 2, "medium": 5, "high": 10}
	latencyScoreMap := map[string]int64{"high": 1, "medium": 5, "low": 10}

	perfScore := perfScoreMap[node.Labels["performance"]]
	latencyScore := latencyScoreMap[node.Labels["latency"]]

	allocCPU := nodeInfo.Allocatable.MilliCPU
	allocMem := nodeInfo.Allocatable.Memory / (1024 * 1024)

	usedCPU := nodeInfo.Requested.MilliCPU
	usedMem := nodeInfo.Requested.Memory / (1024 * 1024)

	cpuUtil := float64(usedCPU) / float64(allocCPU)
	memUtil := float64(usedMem) / float64(allocMem)

	// Broj podova trenutno na node-u
	numPods := len(nodeInfo.Pods)

	// Penalizacija za jače čvorove dok nisu popunjeni
	penalty := 0.0
	switch node.Labels["performance"] {
	case "low":
		penalty = 0
	case "medium":
		if numPods < 3 {
			penalty = 10
		}
	case "high":
		if numPods < 5 {
			penalty = 20
		}
	}

	// Finalna formula
	score := float64(perfScore)*1.5 + (1.0-cpuUtil)*5 + (1.0-memUtil)*3 + float64(latencyScore)*1.0 - penalty

	// Ograniči score između 0 i 100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	// ✨ Lijepi ispis
	fmt.Printf(
		"[%s] pods: %d | CPU: %dm/%dm (%.1f%%) | Mem: %dMi/%dMi (%.1f%%) | perf=%d, lat=%d, penalty=%.1f → score=%.1f\n",
		nodeName,
		numPods,
		usedCPU, allocCPU, cpuUtil*100,
		usedMem, allocMem, memUtil*100,
		perfScore,
		latencyScore,
		penalty,
		score,
	)

	return int64(score), framework.NewStatus(framework.Success, "")
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
