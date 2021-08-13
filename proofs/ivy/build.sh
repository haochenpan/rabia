#!/bin/bash

IMAGE_NAME=rabia-ivy
IMAGE_TAG=latest

docker build -t ${IMAGE_NAME}:${IMAGE_TAG} .
