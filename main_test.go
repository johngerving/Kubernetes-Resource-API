package main

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetNodeInfo(t *testing.T) {
	// Create a fake Kubernetes client
	kubeClient := fake.NewClientset()

	// Create two fake nodes, node-1 and node-2
	newNodes := []v1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
			},
			Status: v1.NodeStatus{
				Capacity: v1.ResourceList{
					v1.ResourceCPU:              *resource.NewMilliQuantity(24000, resource.DecimalSI),
					v1.ResourceMemory:           *resource.NewMilliQuantity(5000, resource.DecimalSI),
					v1.ResourceEphemeralStorage: *resource.NewMilliQuantity(32000, resource.DecimalSI),
				},
				Allocatable: v1.ResourceList{
					v1.ResourceCPU:              *resource.NewMilliQuantity(20000, resource.DecimalSI),
					v1.ResourceMemory:           *resource.NewMilliQuantity(4000, resource.DecimalSI),
					v1.ResourceEphemeralStorage: *resource.NewMilliQuantity(28000, resource.DecimalSI),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-2",
			},
			Status: v1.NodeStatus{
				Capacity: v1.ResourceList{
					v1.ResourceCPU:    *resource.NewMilliQuantity(16000, resource.DecimalSI),
					v1.ResourceMemory: *resource.NewMilliQuantity(5000, resource.DecimalSI),
					"nvidia.com/gpu":  *resource.NewQuantity(2, resource.DecimalSI),
				},
				Allocatable: v1.ResourceList{
					v1.ResourceCPU:    *resource.NewMilliQuantity(12000, resource.DecimalSI),
					v1.ResourceMemory: *resource.NewMilliQuantity(4000, resource.DecimalSI),
					"nvidia.com/gpu":  *resource.NewQuantity(2, resource.DecimalSI),
				},
			},
		},
	}

	// Create the nodes with the fake client
	for _, node := range newNodes {
		kubeClient.CoreV1().Nodes().Create(context.TODO(), &node, metav1.CreateOptions{})
	}

	// Create a map of strings to Node struct instances
	nodes := make(map[string]*Node)

	getNodeInfo(kubeClient, nodes)

	// Loop through the nodes added to the cluster
	for _, node := range newNodes {
		// Check that each node in the cluster exists in the Node resources map
		if _, ok := nodes[node.Name]; !ok {
			t.Fatalf(`map[string]*Node does not contain key %v`, node.Name)
		}

		switch {
		// Check the capacity of each resource to make sure it matches the nodes' values
		case !nodes[node.Name].Capacity.Cpu.Equal(node.Status.Capacity.Cpu().DeepCopy()):
			t.Fatalf(`nodes[%v].Capacity.Cpu = %v, want match for %v`, node.Name, &nodes[node.Name].Capacity.Cpu, node.Status.Capacity.Cpu())
		case !nodes[node.Name].Capacity.Memory.Equal(node.Status.Capacity.Memory().DeepCopy()):
			t.Fatalf(`nodes[%v].Capacity.Memory = %v, want match for %v`, node.Name, &nodes[node.Name].Capacity.Memory, node.Status.Capacity.Memory())
		case !nodes[node.Name].Capacity.Ephemeral.Equal(node.Status.Capacity.StorageEphemeral().DeepCopy()):
			t.Fatalf(`nodes[%v].Capacity.Ephemeral = %v, want match for %v`, node.Name, &nodes[node.Name].Capacity.Ephemeral, node.Status.Capacity.StorageEphemeral())

			// Check the allocatable resources to make sure they match the nodes' values
		case !nodes[node.Name].Allocatable.Cpu.Equal(node.Status.Allocatable.Cpu().DeepCopy()):
			t.Fatalf(`nodes[%v].Allocatable.Cpu = %v, want match for %v`, node.Name, &nodes[node.Name].Allocatable.Cpu, node.Status.Allocatable.Cpu())
		case !nodes[node.Name].Allocatable.Memory.Equal(node.Status.Allocatable.Memory().DeepCopy()):
			t.Fatalf(`nodes[%v].Allocatable.Memory = %v, want match for %v`, node.Name, &nodes[node.Name].Allocatable.Memory, node.Status.Allocatable.Memory())
		case !nodes[node.Name].Allocatable.Ephemeral.Equal(node.Status.Allocatable.StorageEphemeral().DeepCopy()):
			t.Fatalf(`nodes[%v].Allocatable.Ephemeral = %v, want match for %v`, node.Name, &nodes[node.Name].Allocatable.Ephemeral, node.Status.Allocatable.StorageEphemeral())
		}

		// Check that the GPU capacity in the Resources struct instance matches the node's GPU capacity
		if gpuCapacity, ok := node.Status.Capacity["nvidia.com/gpu"]; ok {
			if !gpuCapacity.Equal(nodes[node.Name].Capacity.Gpu.DeepCopy()) {
				t.Fatalf(`nodes[%v].Capacity.Gpu = %v, want match for %v`, node.Name, nodes[node.Name].Capacity.Gpu.Value(), gpuCapacity.Value())
			}
		}

		// Check that the GPU allocatable in the Resources struct instance matches the node's GPU allocatable
		if gpuAllocatable, ok := node.Status.Allocatable["nvidia.com/gpu"]; ok {
			if !gpuAllocatable.Equal(nodes[node.Name].Allocatable.Gpu.DeepCopy()) {
				t.Fatalf(`nodes[%v].Allocatable.Gpu = %v, want match for %v`, node.Name, nodes[node.Name].Allocatable.Gpu.Value(), gpuAllocatable.Value())
			}
		}
	}
}
