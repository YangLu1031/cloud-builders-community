# Description
This repository is used to create a GKE Windows builder to build multi-arch containers using Cloud Build.

Customized from [cloud-builders-community/windows-builder](https://github.com/GoogleCloudPlatform/cloud-builders-community/tree/master/windows-builder).

# Maintaining this builder
Update the `versionMap` in [main.go](builder/main.go) when a new SAC or LTSC version comes out.
Currently the included versions are 1909 & ltsc2019.

# Usage
Refer to the builder in your project's `cloudbuild.yaml` and provide the target container image name. It'll spin up ephemeral `n1-standard-1` VMs on Compute Engine to build the container. Your Cloud Build workspace is synchronized to `C:\workspace` at server startup.

```yaml
steps:
- name: 'gcr.io/$PROJECT_ID/gke-windows-builder'
  args: [ '--container-image-name', '<your target container image:tag name>' ]
```

The VM is configured by the builder and then deleted automatically at the end of the build.

You can also provide the VPC, Subnetwork, Region, and Zone parameters to specify where to create the VMs:

```yaml
steps:
- name: 'gcr.io/$PROJECT_ID/gke-windows-builder'
  args: [ '--network', '<network-name>',
          '--subnetwork', '<subnetwork-name>',
          '--region', '<region>',
          '--zone', '<zone>',
          '--container-image-name', '<your target container image:tag name>' ]
```

As the remote copy command provided is very slow if you have a number of files in your workspace, it is also possible to avoid copying the workspace (you can use GCS to copy the workspace instead):

```yaml
steps:
- name: 'gcr.io/$PROJECT_ID/gke-windows-builder'
  args: [ '--network', '<network-name>',
          '--subnetwork', '<subnetwork-name>',
          '--region', '<region>',
          '--zone', '<zone>',
          '--not-copy-workspace',
          '--container-image-name', '<your target container image:tag name>' ]
```

Your server must support Basic Authentication (username and password) and your network must allow access from the internet on TCP port 5986.  Do not submit plaintext passwords in your build configuration: instead, use [encrypted credentials](https://cloud.google.com/cloud-build/docs/securing-builds/use-encrypted-secrets-credentials) secured with Cloud KMS.  In addition, you must clear up your workspace directory after use, and take care to manage concurrent builds.

## Examples

Example builds are provided:

* [example](example) builds a hello world multi-arch Windows container from servercore 1909 & ltsc2019.
