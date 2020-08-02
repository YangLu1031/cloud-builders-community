package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/GoogleCloudPlatform/cloud-builders-community/windows-builder/builder/builder"
	"github.com/masterzen/winrm"
)

var (
	notCopyWorkspace = flag.Bool("not-copy-workspace", false, "If copy workspace or not")
	workspacePath    = flag.String("workspace-path", "/workspace", "The directory to copy data from")
	workspaceBucket  = flag.String("workspace-bucket", "", "The bucket to copy the directory to. Defaults to {project-id}_cloudbuild")
	network          = flag.String("network", "default", "The VPC name to use when creating the Windows server")
	subnetwork       = flag.String("subnetwork", "default", "The Subnetwork name to use when creating the Windows server")
	region           = flag.String("region", "us-central1", "The region name to use when creating the Windows server")
	zone             = flag.String("zone", "us-central1-f", "The zone name to use when creating the Windows server")
	tag              = flag.String("tag", "gcr.io/yluu-gke-dev/test-win-builder:localbuild", "The resulting container tag name")
	// Windows version and GCE container image map
	versionMap = map[string]string{
		"ltsc2019": "windows-cloud/global/images/windows-server-2019-dc-for-containers-v20191210",
		"1909":     "windows-cloud/global/images/windows-server-1909-dc-core-for-containers-v20200609",
	}
)

func main() {
	log.Print("Starting Windows builder")
	flag.Parse()
	// Construct args of the `docker manifest create` command
	manifestCmd := *tag
	var r *builder.Remote
	var s *builder.Server
	var bs *builder.BuilderServer
	step := 0
	// Bring up specific Windows Build Servers
	for ver, image := range versionMap {
		step++
		ctx := context.Background()
		bs = &builder.BuilderServer{
			ImageUrl: &image,
			VPC:      network,
			Subnet:   subnetwork,
			Region:   region,
			Zone:     zone,
		}
		s = builder.NewServer(ctx, bs)
		r = &s.Remote

		log.Print("Waiting for server to become available")
		err := r.Wait()
		if err != nil {
			log.Fatalf("Error connecting to server: %+v", err)
		}

		r.BucketName = workspaceBucket
		// Copy workspace to remote machine
		if !*notCopyWorkspace {
			log.Print("Copying workspace")
			err = r.Copy(*workspacePath)
			if err != nil {
				log.Fatalf("Error copying workspace: %+v", err)
			}
		}

		// Build single arch container on remote
		buildSingleArchContainerScript := fmt.Sprintf(`
		$env:DOCKER_CLI_EXPERIMENTAL = 'enabled'
		gcloud --quiet auth configure-docker
		docker build -t %[1]s_%[2]s --build-arg version=%[2]s .
		docker push %[1]s_%[2]s
		`, *tag, ver)

		err = r.Run(winrm.Powershell(buildSingleArchContainerScript))
		if err != nil {
			log.Fatalf("Error executing buildSingleArchContainerScript: %+v", err)
		}

		manifestCmd += fmt.Sprint(" ", *tag, "_", ver)
		// If it's last step, build multi-arch container on remote
		if step == len(versionMap) {
			createMultiarchContainerScript := fmt.Sprintf(`
			$env:DOCKER_CLI_EXPERIMENTAL = 'enabled'
			gcloud --quiet auth configure-docker
			docker manifest create %s
			docker manifest push %s
			`, manifestCmd, *tag)

			err = r.Run(winrm.Powershell(createMultiarchContainerScript))
			if err != nil {
				log.Fatalf("Error executing createMultiarchContainerScript: %+v", err)
			}
		}

		// Shut down server if started
		if s != nil {
			err = s.DeleteInstance(bs)
			if err != nil {
				log.Fatalf("Failed to shut down instance: %+v", err)
			}
		}
	}
}
