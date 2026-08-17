package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"

	"github.com/altshift/terraform-provider-gws/internal/provider"
	altshiftHttpLogger "github.com/altshiftab/utils_go/pkg/log/http_logger"
	"github.com/altshiftab/utils_go/pkg/log/http_logger/http_logger_config"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var version = "dev"

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	logger := altshiftHttpLogger.New(
		http_logger_config.WithLogLevel(slog.LevelDebug),
		http_logger_config.WithWriter(os.Stderr),
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
