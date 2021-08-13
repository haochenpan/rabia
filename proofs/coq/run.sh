#!/bin/bash

IMAGE_NAME=rabia-coq
IMAGE_TAG=latest

docker build -t ${IMAGE_NAME}:${IMAGE_TAG} .

docker run -it --volume="/tmp/.X11-unix:/tmp/.X11-unix:rw" -e DISPLAY=$DISPLAY ${IMAGE_NAME}:${IMAGE_TAG} coqc weak_mvc.v
