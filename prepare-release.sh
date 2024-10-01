#!/bin/bash

export VERSION=$1

yq chart/values.yaml > chart/values.yaml.old_tag
yq '.image.tag = env(VERSION)' chart/values.yaml > chart/values.yaml.new_tag

diff chart/values.yaml.old_tag chart/values.yaml.new_tag > chart/values.yaml.diff
patch chart/values.yaml < chart/values.yaml.diff

rm chart/values.yaml.*


yq chart/Chart.yaml > chart/Chart.yaml.old_tag
yq '.version = env(VERSION) | .appVersion = env(VERSION)' chart/Chart.yaml > chart/Chart.yaml.new_tag

diff chart/Chart.yaml.old_tag chart/Chart.yaml.new_tag > chart/Chart.yaml.diff
patch chart/Chart.yaml < chart/Chart.yaml.diff

rm chart/Chart.yaml.*