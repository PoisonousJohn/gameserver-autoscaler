package main

import (
	"context"

	azBatch "github.com/Azure/azure-sdk-for-go/services/batch/2017-09-01.6.0/batch"
	"github.com/PoisonousJohn/gameserver-autoscaler/batch"
)

// NodeInfo represents basic info about Node in Azure Batch
// pools, required to track its state
type NodeInfo struct {
	ID       string
	acc      batch.Account
	publicIP string
}

func toNodeInfo(acc batch.Account, node azBatch.ComputeNode) NodeInfo {
	var publicIP string
	endpoints := node.EndpointConfiguration.InboundEndpoints
	if endpoints != nil && len(*endpoints) > 0 {
		publicIP = *(*endpoints)[0].PublicIPAddress
	}
	return NodeInfo{
		ID:       *node.ID,
		acc:      acc,
		publicIP: publicIP,
	}
}

func getAccountNodesInfo(ctx context.Context, acc batch.Account, resultsChan chan<- []NodeInfo) {
	var result []NodeInfo
	defer func() { resultsChan <- result }()
	nodes, err := batch.GetPoolNodes(ctx, acc, poolID)
	if err != nil {
		return
	}

	result = make([]NodeInfo, len(nodes))
	for index, node := range nodes {
		result[index] = toNodeInfo(acc, node)
	}
}

func getNodesInfo(ctx context.Context, conf AppConf) []NodeInfo {
	accs := len(conf.BatchAccounts)
	resultsChan := make(chan []NodeInfo, accs)
	for _, acc := range conf.BatchAccounts {
		go getAccountNodesInfo(ctx, acc, resultsChan)
	}

	result := make([]NodeInfo, 0)
	for i := 0; i < accs; i++ {
		nodes := <-resultsChan
		for _, node := range nodes {
			result = append(result, node)
		}
	}
	return result
}
