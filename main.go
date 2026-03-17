package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"

	"github.com/altshift/terraform-provider-gws/internal/provider"
	gcpUtilsLogger "github.com/altshiftab/gcp_utils/pkg/types/logger"
	"github.com/altshiftab/gcp_utils/pkg/types/logger/logger_config"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var version = "dev"

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	// TODO: Fix the logger.
	logger := gcpUtilsLogger.New(
		logger_config.WithLogLevel(slog.LevelDebug),
		logger_config.WithWriter(os.Stderr),
	)
	slog.SetDefault(logger.Logger)

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/altshift/gws",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
