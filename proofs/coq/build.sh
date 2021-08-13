#!/bin/bash

IMAGE_NAME=rabia-coq
IMAGE_TAG=latest

docker build -t ${IMAGE_NAME}:${IMAGE_TAG} .

