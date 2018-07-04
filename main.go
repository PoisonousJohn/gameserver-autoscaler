package main

import (
	"context"
	"net/http"

	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/Azure-Samples/azure-sdk-for-go-samples/iam"
	"gopkg.in/julienschmidt/httprouter.v1"

	"gopkg.in/yaml.v2"
)

var (
	logger = log.New(os.Stdout, "[main] ", log.Lshortfile)
)

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

	router := httprouter.New()
	router.POST("/instance", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		createServiceInstanceHandler(ctx, config, w, r)
	})

	log.Fatal(http.ListenAndServe(":8080", router))
}
