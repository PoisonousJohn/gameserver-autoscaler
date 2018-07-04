// Copyright (c) Microsoft and contributors.  All rights reserved.
//
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package batch

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Azure-Samples/azure-sdk-for-go-samples/helpers"
	"github.com/Azure-Samples/azure-sdk-for-go-samples/iam"
	"github.com/Azure/azure-sdk-for-go/services/batch/2017-09-01.6.0/batch"
	batchARM "github.com/Azure/azure-sdk-for-go/services/batch/mgmt/2017-09-01/batch"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	uuid "github.com/satori/go.uuid"
)

var (
	logger = log.New(os.Stdout, "[batch] ", log.Lshortfile)
)

// Account represents basic credentials for the batch account
type Account struct {
	Name     string `yaml:"Name"`
	Location string `yaml:"Location"`
}

const (
	stdoutFile string = "stdout.txt"
	stderrFile string = "stderr.txt"
)

func getAccountClient() batchARM.AccountClient {
	accountClient := batchARM.NewAccountClient(helpers.SubscriptionID())
	auth, _ := iam.GetResourceManagementAuthorizer(iam.AuthGrantType())
	accountClient.Authorizer = auth
	accountClient.AddToUserAgent(helpers.UserAgent())
	return accountClient
}

func getPoolClient(acc Account) batch.PoolClient {
	poolClient := batch.NewPoolClientWithBaseURI(getBatchBaseURL(acc))
	auth, _ := iam.GetBatchAuthorizer(iam.AuthGrantType())
	poolClient.Authorizer = auth
	poolClient.AddToUserAgent(helpers.UserAgent())
	poolClient.RequestInspector = fixContentTypeInspector()
	return poolClient
}

func getJobClient(acc Account) batch.JobClient {
	jobClient := batch.NewJobClientWithBaseURI(getBatchBaseURL(acc))
	auth, _ := iam.GetBatchAuthorizer(iam.AuthGrantType())
	jobClient.Authorizer = auth
	jobClient.AddToUserAgent(helpers.UserAgent())
	// jobClient.RequestInspector = fixContentTypeInspector()
	return jobClient
}

func getTaskClient(acc Account) batch.TaskClient {
	taskClient := batch.NewTaskClientWithBaseURI(getBatchBaseURL(acc))
	auth, _ := iam.GetBatchAuthorizer(iam.AuthGrantType())
	taskClient.Authorizer = auth
	taskClient.AddToUserAgent(helpers.UserAgent())
	taskClient.RequestInspector = fixContentTypeInspector()
	return taskClient
}

func getFileClient(acc Account) batch.FileClient {
	fileClient := batch.NewFileClientWithBaseURI(getBatchBaseURL(acc))
	auth, _ := iam.GetBatchAuthorizer(iam.AuthGrantType())
	fileClient.Authorizer = auth
	fileClient.AddToUserAgent(helpers.UserAgent())
	fileClient.RequestInspector = fixContentTypeInspector()
	return fileClient
}

func getComputeNodeClient(acc Account) batch.ComputeNodeClient {
	computeNodeClient := batch.NewComputeNodeClientWithBaseURI(getBatchBaseURL(acc))
	auth, _ := iam.GetBatchAuthorizer(iam.AuthGrantType())
	computeNodeClient.Authorizer = auth
	computeNodeClient.AddToUserAgent(helpers.UserAgent())
	computeNodeClient.RequestInspector = fixContentTypeInspector()
	return computeNodeClient
}

// CreateBatchPool creates an Azure Batch compute pool
func CreateBatchPool(ctx context.Context, acc Account, poolID string) error {
	poolClient := getPoolClient(acc)
	toCreate := batch.PoolAddParameter{
		ID: &poolID,
		VirtualMachineConfiguration: &batch.VirtualMachineConfiguration{
			ImageReference: &batch.ImageReference{
				Publisher: to.StringPtr("Canonical"),
				Sku:       to.StringPtr("16.04-LTS"),
				Offer:     to.StringPtr("UbuntuServer"),
				Version:   to.StringPtr("latest"),
			},
			NodeAgentSKUID: to.StringPtr("batch.node.ubuntu 16.04"),
		},
		MaxTasksPerNode:      to.Int32Ptr(1),
		TargetDedicatedNodes: to.Int32Ptr(0),
		// Create a startup task to run a script on each pool machine
		StartTask: &batch.StartTask{
			ResourceFiles: &[]batch.ResourceFile{
				{
					BlobSource: to.StringPtr("https://raw.githubusercontent.com/lawrencegripper/azure-sdk-for-go-samples/1441a1dc4a6f7e47c4f6d8b537cf77ce4f7c452c/batch/examplestartup.sh"),
					FilePath:   to.StringPtr("echohello.sh"),
					FileMode:   to.StringPtr("777"),
				},
			},
			CommandLine:    to.StringPtr("bash -f echohello.sh"),
			WaitForSuccess: to.BoolPtr(true),
			UserIdentity: &batch.UserIdentity{
				AutoUser: &batch.AutoUserSpecification{
					ElevationLevel: batch.Admin,
					Scope:          batch.Task,
				},
			},
		},
		VMSize: to.StringPtr("standard_a1"),
	}

	_, err := poolClient.Add(ctx, toCreate, nil, nil, nil, nil)

	return err
}

// CreateBatchJob creates an azure batch job
func CreateBatchJob(ctx context.Context, acc Account, poolID, jobID string) error {
	jobClient := getJobClient(acc)
	jobToCreate := batch.JobAddParameter{
		ID: to.StringPtr(jobID),
		PoolInfo: &batch.PoolInformation{
			PoolID: to.StringPtr(poolID),
		},
	}
	_, err := jobClient.Add(ctx, jobToCreate, nil, nil, nil, nil)

	return err
}

// GetPoolNodes returns an []batch.ComputeNode for the specificed account
func GetPoolNodes(ctx context.Context, acc Account, poolID string) ([]batch.ComputeNode, error) {
	client := getComputeNodeClient(acc)
	result, err := client.List(ctx, poolID, "", "", nil, nil, nil, nil, nil)

	return result.Values(), err
}

// CreateBatchTask creates a task with specified command
func CreateBatchTask(ctx context.Context, acc Account, jobID, cmd string) (string, error) {
	newUUID, _ := uuid.NewV4()
	taskID := newUUID.String()
	taskClient := getTaskClient(acc)
	taskToAdd := batch.TaskAddParameter{
		ID:          &taskID,
		CommandLine: &cmd,
		UserIdentity: &batch.UserIdentity{
			AutoUser: &batch.AutoUserSpecification{
				ElevationLevel: batch.Admin,
				Scope:          batch.Task,
			},
		},
	}
	_, err := taskClient.Add(ctx, jobID, taskToAdd, nil, nil, nil, nil)

	if err != nil {
		return "", err
	}

	return taskID, nil
}

// EnsurePoolExists creates the pool required for running game servers, if it doesn't exist
func EnsurePoolExists(ctx context.Context, acc Account, poolID string) error {
	logger.Printf("Creating pool")
	err := CreateBatchPool(ctx, acc, poolID)
	if err != nil {
		detailedErr := err.(autorest.DetailedError)

		// pool already exists
		if detailedErr.StatusCode == 409 {
			err = nil
		}
	}
	if err == nil {
		logger.Printf("Pool created")
	}

	return err
}

// EnsureJobExists creates the job required for running game servers, if it doesn't exist
func EnsureJobExists(ctx context.Context, acc Account, poolID string, jobID string) error {
	logger.Printf("Creating job")
	err := CreateBatchJob(ctx, acc, poolID, jobID)

	if err != nil {
		detailedErr := err.(autorest.DetailedError)

		// pool already exists
		if detailedErr.StatusCode == 409 {
			err = nil
		}
	}

	if err == nil {
		logger.Printf("Job created")
	}

	return err
}

// DeleteTask well... deletes the task. If the task doesn't exist, it's not an error
func DeleteTask(ctx context.Context, acc Account, jobID, taskID string) error {
	taskClient := getTaskClient(acc)
	resp, err := taskClient.Delete(ctx, jobID, taskID, nil, nil, nil, nil, "", "", nil, nil)

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	return err
}

// GetTask returns a task
func GetTask(ctx context.Context, acc Account, jobID, taskID string) (batch.CloudTask, error) {
	taskClient := getTaskClient(acc)
	return taskClient.Get(ctx, jobID, taskID, "", "", nil, nil, nil, nil, "", "", nil, nil)
}

// WaitForTaskResult polls the task and retreives it's stdout once it has completed
func WaitForTaskResult(ctx context.Context, acc Account, jobID, taskID string) (stdout string, err error) {
	taskClient := getTaskClient(acc)
	res, err := taskClient.Get(ctx, jobID, taskID, "", "", nil, nil, nil, nil, "", "", nil, nil)
	if err != nil {
		return "", err
	}
	waitCtx, cancel := context.WithTimeout(ctx, time.Minute*4)
	defer cancel()

	if res.State != batch.TaskStateCompleted {
		for {
			_, ok := waitCtx.Deadline()
			if !ok {
				return stdout, errors.New("timedout waiting for task to execute")
			}
			time.Sleep(time.Second * 15)
			res, err = taskClient.Get(ctx, jobID, taskID, "", "", nil, nil, nil, nil, "", "", nil, nil)
			if err != nil {
				return "", err
			}
			if res.State == batch.TaskStateCompleted {
				waitCtx.Done()
				break
			}
		}
	}

	fileClient := getFileClient(acc)

	reader, err := fileClient.GetFromTask(ctx, jobID, taskID, stdoutFile, nil, nil, nil, nil, "", nil, nil)

	if err != nil {
		return "", err
	}

	bytes, err := ioutil.ReadAll(*reader.Value)

	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func getBatchBaseURL(acc Account) string {
	return fmt.Sprintf("https://%s.%s.batch.azure.com", acc.Name, acc.Location)
}

// This is required due to this issue: https://github.com/Azure/azure-sdk-for-go/issues/1159. Can be removed once resolved.
func fixContentTypeInspector() autorest.PrepareDecorator {
	return func(p autorest.Preparer) autorest.Preparer {
		return autorest.PreparerFunc(func(r *http.Request) (*http.Request, error) {
			r, err := p.Prepare(r)
			if err == nil {
				r.Header.Set("Content-Type", "application/json; odata=minimalmetadata")
			}
			return r, nil
		})
	}
}

// CreateAzureBatchAccount creates a new azure batch account
// func CreateAzureBatchAccount(ctx context.Context, accountName, location, resourceGroupName string) (a batchARM.Account, err error) {
// 	accountClient := getAccountClient()
// 	res, err := accountClient.Create(ctx, resourceGroupName, accountName, batchARM.AccountCreateParameters{
// 		Location: to.StringPtr(location),
// 	})

// 	if err != nil {
// 		return a, err
// 	}

// 	err = res.WaitForCompletion(ctx, accountClient.Client)

// 	if err != nil {
// 		return batchARM.Account{}, fmt.Errorf("failed waiting for account creation: %v", err)
// 	}

// 	account, err := res.Result(accountClient)

// 	if err != nil {
// 		return a, fmt.Errorf("failed retreiving for account: %v", err)
// 	}

// 	return account, nil
// }
