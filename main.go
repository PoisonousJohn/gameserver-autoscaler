package main

import (
	"context"
	"errors"
	"sync"

	"github.com/Azure-Samples/azure-sdk-for-go-samples/iam"
	"github.com/Azure/go-autorest/autorest"
	"github.com/PoisonousJohn/gameserver-autoscaler/batch"
	// "github.com/subosito/gotenv"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// AppConf represents a basic config for this app
type AppConf struct {
	AzureSubscriptionID string          `yaml:"AzureSubscriptionID,omitempty"`
	AzureTenantID       string          `yaml:"AzureTenantID,omitempty"`
	AzureClientID       string          `yaml:"AzureClientID,omitempty"`
	AzureClientSecret   string          `yaml:"AzureClientSecret,omitempty"`
	BatchAccounts       []batch.Account `yaml:"BatchAccounts"`
}

// NodeInfo represents basic info about Node in Azure Batch
// pools, required to track its state
type NodeInfo struct {
	ID       string
	acc      batch.Account
	publicIP string
}

const jobID = "GameServers"
const poolID = "GameServers"

var (
	jobCreated  = false
	poolCreated = false
	logger      = log.New(os.Stdout, "[main] ", log.Lshortfile)
)

func getBatchAccConf() (batch.Account, error) {
	conf := batch.Account{
		Name:     os.Getenv("BATCH_ACCOUNT"),
		Location: os.Getenv("BATCH_LOCATION"),
	}
	if len(conf.Name) == 0 {
		return conf, errors.New("BATCH_ACCOUNT env variable should be set")
	}

	if len(conf.Location) == 0 {
		return conf, errors.New("BATCH_LOCATION env variable should be set")
	}

	return conf, nil
}

func ensurePoolExists(ctx context.Context, acc batch.Account) error {
	if poolCreated {
		return nil
	}

	logger.Printf("Creating pool")
	err := batch.CreateBatchPool(ctx, acc, poolID)
	if err != nil {
		detailedErr := err.(autorest.DetailedError)

		// pool already exists
		if detailedErr.StatusCode == 409 {
			err = nil
		}
	}
	if err == nil {
		poolCreated = true
		logger.Printf("Pool created")
	}
	return err
}

func ensureJobExists(ctx context.Context, acc batch.Account) error {
	if jobCreated {
		return nil
	}

	logger.Printf("Creating job")
	err := batch.CreateBatchJob(ctx, acc, poolID, jobID)

	if err != nil {
		detailedErr := err.(autorest.DetailedError)

		// pool already exists
		if detailedErr.StatusCode == 409 {
			err = nil
		}
	}

	if err == nil {
		jobCreated = true
		logger.Printf("Job created")
	}

	return err
}

func setAuthEnvVars(conf AppConf) {
	os.Setenv("AZURE_SUBSCRIPTION_ID", conf.AzureSubscriptionID)
	os.Setenv("AZURE_TENANT_ID", conf.AzureTenantID)
	os.Setenv("AZURE_CLIENT_ID", conf.AzureClientID)
	os.Setenv("AZURE_CLIENT_SECRET", conf.AzureClientSecret)
}

func getNodesInfo(ctx context.Context, conf AppConf) []NodeInfo {
	var wg sync.WaitGroup
	accs := len(conf.BatchAccounts)
	resultsChan := make(chan []NodeInfo, accs)
	for _, acc := range conf.BatchAccounts {
		wg.Add(1)
		account := acc
		go func() {
			var result []NodeInfo
			defer func() { resultsChan <- result }()
			defer wg.Done()
			nodes, err := batch.GetPoolNodes(ctx, account, poolID)
			if err != nil {
				return
			}

			result = make([]NodeInfo, len(nodes))
			for index, node := range nodes {
				var publicIP string
				endpoints := node.EndpointConfiguration.InboundEndpoints
				if endpoints != nil && len(*endpoints) > 0 {
					publicIP = *(*endpoints)[0].PublicIPAddress
				}
				result[index] = NodeInfo{
					ID:       *node.ID,
					acc:      account,
					publicIP: publicIP,
				}
			}
		}()
	}

	wg.Wait()
	result := make([]NodeInfo, 0)
	for i := 0; i < accs; i++ {
		nodes := <-resultsChan
		for _, node := range nodes {
			result = append(result, node)
		}
	}
	return result
}

func createServerInstance(ctx context.Context, acc batch.Account) error {
	err := ensurePoolExists(ctx, acc)
	if err != nil {
		logger.Panicf("Failed to create pool: %s", err.Error())
		return err
	}

	err = ensureJobExists(ctx, acc)

	if err != nil {
		logger.Panicf("Failed to create job: %s", err.Error())
		return err
	}
	logger.Printf("creating server instance")
	_, err = batch.CreateBatchTask(ctx, acc, jobID, "/bin/bash -c 'echo \"hello\"'")
	return err
}

func main() {

	filename, _ := filepath.Abs("./appsettings.yml")
	yamlFile, err := ioutil.ReadFile(filename)

	if err != nil {
		panic(err)
	}

	var config AppConf
	err = yaml.Unmarshal(yamlFile, &config)
	setAuthEnvVars(config)

	if err = iam.ParseArgs(); err != nil {
		logger.Panicf("Failed to parse OAuth settings: %s", err)
		return
	}

	if err != nil {
		logger.Panicf("Batch config is invalid: %s", err.Error())
		return
	}

	ctx := context.Background()

	info := getNodesInfo(ctx, config)
	for _, node := range info {
		logger.Printf("Node: %s -> %s", node.ID, node.publicIP)
	}

	// if err = createServerInstance(ctx, config.BatchAccounts[0]); err != nil {
	// 	// if err = createServerInstance(ctx, batch.Account{}); err != nil {
	// 	logger.Panicf("Failed to create server instance: %s", err.Error())
	// 	return
	// }

	// logger.Printf("Server instance created")
}
