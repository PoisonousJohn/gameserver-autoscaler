package main

import (
	"context"
	"errors"
	"github.com/Azure-Samples/azure-sdk-for-go-samples/iam"
	"github.com/Azure/go-autorest/autorest"
	"github.com/PoisonousJohn/gameserver-autoscaler/batch"
	"github.com/subosito/gotenv"
	"log"
	"os"
)

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
	if err := gotenv.Load("appsettings.env"); err != nil {
		logger.Panicf("Failed to load env file: %s", err.Error())
	}

	acc, err := getBatchAccConf()
	if err = iam.ParseArgs(); err != nil {
		logger.Panicf("Failed to parse OAuth settings: %s", err)
		return
	}

	if err != nil {
		logger.Panicf("Batch config is invalid: %s", err.Error())
		return
	}

	ctx := context.Background()
	if err = createServerInstance(ctx, acc); err != nil {
		logger.Panicf("Failed to create server instance: %s", err.Error())
		return
	}

	logger.Printf("Server instance created")
}
