#!/bin/bash
# gcloud compute images li÷st --filter="name:(1909-dc-core-for-containers)" --project=windows-cloud --format="value(name)"
gcloud builds submit --config=windows-builder/images/multi-arch_1909_2019/multiarchBuild.yaml --substitutions=_VERSION1=ltsc2019,_VERSION2=1909