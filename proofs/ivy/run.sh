#!/bin/bash

IMAGE_NAME=ivytest
IMAGE_TAG=latest

docker build -t ${IMAGE_NAME}:${IMAGE_TAG} .

docker run -it --volume="/tmp/.X11-unix:/tmp/.X11-unix:rw" -e DISPLAY=$DISPLAY ${IMAGE_NAME}:${IMAGE_TAG} ivy_check macro_finder=false seed=1 isolate=protocol trace=true weak_mvc.ivy
