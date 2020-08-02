package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"windows-cloudbuild-builder/builder/builder"

	"github.com/masterzen/winrm"
)

var (
	workspacePath      = flag.String("workspace-path", "/workspace", "The directory to copy data from")
	workspaceBucket    = flag.String("workspace-bucket", "", "The bucket to copy the directory to. Defaults to {project-id}_cloudbuild")
	network            = flag.String("network", "default", "The VPC name to use when creating the Windows Instance")
	subnetwork         = flag.String("subnetwork", "default", "The Subnetwork name to use when creating the Windows Instance")
	region             = flag.String("region", "us-central1", "The region name to use when creating the Windows Instance")
	zone               = flag.String("zone", "us-central1-f", "The zone name to use when creating the Windows Instance")
	labels             = flag.String("labels", "", "List of label KEY=VALUE pairs separated by comma to add when creating the Windows Instance")
	machineType        = flag.String("machineType", "", "The machine type to use when creating the Windows Instance")
	copyTimeout        = flag.Int("copyTimeout", 5, "The workspace copy timeout in minutes")
	serviceAccount     = flag.String("serviceAccount", "default", "The service account to use when creating the Windows Instance")
	containerImageName = flag.String("container-image-name", "", "The target container image:tag name")
	// Windows version and GCE container image map
	// the version name need to match with servercore container version in Dockerfile file
	versionMap = map[string]string{
		"ltsc2019": "windows-cloud/global/images/windows-server-2019-dc-for-containers-v20191210",
		"1909":     "windows-cloud/global/images/windows-server-1909-dc-core-for-containers-v20200609",
	}
	commandTimeout = 10
)

func main() {
	log.Print("Starting Windows multi-arch container builder")
	flag.Parse()
	if *containerImageName == "" {
		log.Fatalf("Error container-image-name flag is required but was not set")
	}
	var r *builder.Remote
	var s *builder.Server
	var bs *builder.BuilderServer
	step := 0
	// Bring up specific Windows Build Servers
	for ver, image := range versionMap {
		step++
		ctx := context.Background()
		bs = &builder.BuilderServer{
			ImageUrl:       &image,
			VPC:            network,
			Subnet:         subnetwork,
			Region:         region,
			Zone:           zone,
			Labels:         labels,
			MachineType:    machineType,
			ServiceAccount: serviceAccount,
		}
		s = builder.NewServer(ctx, bs)
		r = &s.Remote

		log.Printf("Waiting for Windows %s instance: %s to become available", ver, *r.Hostname)
		err := r.Wait()
		if err != nil {
			log.Printf("Error connecting to Windows %s instance: %s with error: %+v", ver, *r.Hostname, err)
			deleteInstance(s, bs)
			os.Exit(1)
		}

		r.BucketName = workspaceBucket
		// Copy workspace to remote machine
		log.Print("Copying local workspace to remote machine")
		err = r.Copy(*workspacePath, *copyTimeout)
		if err != nil {
			log.Printf("Error copying workspace: %+v", err)
			deleteInstance(s, bs)
			os.Exit(1)
		}

		buildSingleArchContainerOnRemote(r, *containerImageName, ver, commandTimeout, s, bs)

		// If it's last step, build multi-arch container on remote before shut down the VM.
		if step == len(versionMap) {
			manifestCreateCmdArgs := constructArgsOfManifestCreateCommand()
			createMultiArchContainerOnRemote(r, *containerImageName, manifestCreateCmdArgs, commandTimeout, s, bs)
		}

		// Shut down server if started
		deleteInstance(s, bs)
	}
}

// Construct the args of `docker manifest create` cmd
// e.g. `docker manifest create demo:cloudbuild demo:cloudbuild_ltsc2019 demo:cloudbuild_1909`
func constructArgsOfManifestCreateCommand() string {
	args := *containerImageName
	for ver := range versionMap {
		args += fmt.Sprint(" ", *containerImageName, "_", ver)
	}
	return args
}

func buildSingleArchContainerOnRemote(
	r *builder.Remote,
	containerImageName string,
	version string,
	timeoutInMinutes int,
	s *builder.Server,
	bs *builder.BuilderServer,
) {
	buildSingleArchContainerScript := fmt.Sprintf(`
	$env:DOCKER_CLI_EXPERIMENTAL = 'enabled'
	gcloud --quiet auth configure-docker
	docker build -t %[1]s_%[2]s --build-arg version=%[2]s .
	docker push %[1]s_%[2]s
	`, containerImageName, version)

	log.Printf("Start to build single-arch container with commands: %s", buildSingleArchContainerScript)
	err := r.Run(winrm.Powershell(buildSingleArchContainerScript), timeoutInMinutes)
	if err != nil {
		log.Printf("Error executing buildSingleArchContainerScript: %+v", err)
		deleteInstance(s, bs)
		os.Exit(1)
	}
}

func createMultiArchContainerOnRemote(
	r *builder.Remote,
	containerImageName string,
	manifestCreateCmdArgs string,
	timeoutInMinutes int,
	s *builder.Server,
	bs *builder.BuilderServer,
) {
	createMultiarchContainerScript := fmt.Sprintf(`
	$env:DOCKER_CLI_EXPERIMENTAL = 'enabled'
	gcloud --quiet auth configure-docker
	docker manifest create %s
	docker manifest push %s
	`, manifestCreateCmdArgs, containerImageName)

	log.Printf("Start to create multi-arch container with commands: %s", createMultiarchContainerScript)
	err := r.Run(winrm.Powershell(createMultiarchContainerScript), timeoutInMinutes)
	if err != nil {
		log.Printf("Error executing createMultiarchContainerScript: %+v", err)
		deleteInstance(s, bs)
		os.Exit(1)
	}
}

func deleteInstance(s *builder.Server, bs *builder.BuilderServer) {
	err := s.DeleteInstance(bs)
	if err != nil {
		log.Fatalf("Failed to shut down instance: %s, with error: %+v", *s.Remote.Hostname, err)
	} else {
		log.Printf("Instance: %s shut down successfully", *s.Remote.Hostname)
	}
}
