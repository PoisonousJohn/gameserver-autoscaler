package main

import (
	"os"

	"github.com/PoisonousJohn/gameserver-autoscaler/batch"
)

// AppConf represents a basic config for this app
type AppConf struct {
	AzureSubscriptionID string          `yaml:"AzureSubscriptionID,omitempty"`
	AzureTenantID       string          `yaml:"AzureTenantID,omitempty"`
	AzureClientID       string          `yaml:"AzureClientID,omitempty"`
	AzureClientSecret   string          `yaml:"AzureClientSecret,omitempty"`
	BatchAccounts       []batch.Account `yaml:"BatchAccounts"`
}

func setAuthEnvVars(conf AppConf) {
	os.Setenv("AZURE_SUBSCRIPTION_ID", conf.AzureSubscriptionID)
	os.Setenv("AZURE_TENANT_ID", conf.AzureTenantID)
	os.Setenv("AZURE_CLIENT_ID", conf.AzureClientID)
	os.Setenv("AZURE_CLIENT_SECRET", conf.AzureClientSecret)
}
