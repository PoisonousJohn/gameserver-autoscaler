package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	azBatch "github.com/Azure/azure-sdk-for-go/services/batch/2017-09-01.6.0/batch"
	"github.com/PoisonousJohn/gameserver-autoscaler/batch"
)

const jobID = "GameServers"
const poolID = "GameServers"

var (
	jobCreated  = false
	poolCreated = false
	// key is a node id
	nodesInfo = make(map[string]NodeInfo)
)

// CreateServerInstanceRequest represents a struct for the request to create a server instance
type CreateServerInstanceRequest struct {
	Region string
}

// CreateServerInstanceResponse represents a struct for the response
// of creating a server instance request
type CreateServerInstanceResponse struct {
	PublicIP string `json:"publicIP"`
	Port     string `json:"port"`
}

func (c AppConf) getAccForRegion(region string) *batch.Account {
	for _, acc := range c.BatchAccounts {
		if acc.Location == region {
			return &acc
		}
	}

	return nil
}

func updateNodesInfo(ctx context.Context, conf AppConf) {
	nodes := getNodesInfo(ctx, conf)
	for _, node := range nodes {
		logger.Printf("Found node %s", node.ID)
		nodesInfo[node.ID] = node
	}

	logger.Printf("Updated nodes info. Total count %d", len(nodesInfo))
}

func getTaskNode(ctx context.Context, conf AppConf, acc batch.Account, taskID string) (*NodeInfo, error) {

	var result azBatch.CloudTask
	var err error
	waitCtx, cancel := context.WithTimeout(ctx, time.Minute*3)
	defer cancel()
	for {

		result, err = batch.GetTask(waitCtx, acc, jobID, taskID)

		if err != nil {
			return nil, err
		}

		if result.State == azBatch.TaskStateRunning &&
			result.NodeInfo != nil &&
			result.NodeInfo.NodeID != nil {
			break
		}

		select {
		case <-waitCtx.Done():
			logger.Println("Scheduling the task to a node is timed out, deleting the task")
			err = batch.DeleteTask(ctx, acc, jobID, taskID)
			if err != nil {
				return nil, err
			}
			return nil, errors.New("Creating the service timed out")
		case <-time.After(time.Millisecond * 500):
			logger.Println("Task is not on the node, waiting")
		}
	}

	nodeInfo, exists := nodesInfo[*result.NodeInfo.NodeID]

	if !exists {
		logger.Printf("node %s was not found, updating", *result.NodeInfo.NodeID)
		updateNodesInfo(ctx, conf)
	}

	nodeInfo, exists = nodesInfo[*result.NodeInfo.NodeID]

	if !exists {
		return nil, errors.New("Task id refers to the node that was not found")
	}

	return &nodeInfo, nil
}

func createServiceInstanceHandler(ctx context.Context, conf AppConf, w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	d := json.NewDecoder(r.Body)
	var reqObj CreateServerInstanceRequest
	err := d.Decode(&reqObj)
	if err != nil {
		http.Error(w, "Can't decode request", http.StatusBadRequest)
		return
	}

	acc := conf.getAccForRegion(reqObj.Region)
	if acc == nil {
		http.Error(w, "Couldn't find appropriate account for this region", http.StatusInternalServerError)
		return
	}

	logger.Println("Checking the pool")
	if !poolCreated {
		err := batch.EnsurePoolExists(ctx, *acc, poolID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create a pool: %s", err), http.StatusInternalServerError)
			return
		}
		poolCreated = true
	}

	logger.Println("Checking the job")
	if !jobCreated {
		err := batch.EnsureJobExists(ctx, *acc, poolID, jobID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create a job: %s", err), http.StatusInternalServerError)
			return
		}
		jobCreated = true
	}

	logger.Println("Creating the task")
	taskID, err := batch.CreateBatchTask(ctx, *acc, jobID, "/bin/bash -c 'sleep 10'")

	logger.Println("Getting task's node")
	var node *NodeInfo
	node, err = getTaskNode(ctx, conf, *acc, taskID)

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get server ip: %s", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp := CreateServerInstanceResponse{
		PublicIP: node.publicIP,
	}
	json.NewEncoder(w).Encode(&resp)

	elapsedTime := time.Since(startTime)
	logger.Printf("Created server instance in %s", elapsedTime)
}
