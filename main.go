package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/Spokane-Mountaineers/terraform-provider-google-workspace/internal/provider"
)

// version is set via -ldflags at release time; "dev" otherwise.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		// The repo is terraform-provider-google-workspace, but the resource type
		// prefix cannot contain hyphens, so resources are googleworkspace_*.
		Address: "registry.spokanemountaineers.org/spokane-mountaineers/google-workspace",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err.Error())
	}
}
