#!/bin/bash

# We want to use some private functinos defined here
source /home/ubuntu/.functions.sh

while [[ "$#" -gt 0 ]]; do
  case $1 in
    -b|--build) build=1 ;;
    -u|--upload) upload=1 ;;
    *) echo "Unknown parameter passed: $1"; exit 1 ;;
  esac
  shift
done

build_all() {
  echo "Building all services..."
  PARENT_DIR=$(dirname "$(realpath "$0")")

  cd $PARENT_DIR/../svc-1-data-collector
  docker build -t svc-1-data-collector .

  cd $PARENT_DIR/../svc-2-data-ingestor
  docker build -t svc-2-data-ingestor .

  cd $PARENT_DIR/../svc-3-local-storage
  docker build -t svc-3-local-storage .

  cd $PARENT_DIR/../svc-4-data-processor
  docker build -t svc-4-data-processor .

  cd $PARENT_DIR/../svc-5-central-storage
  docker build -t svc-5-central-storage .

  cd $PARENT_DIR/../svc-6-dashboard
  docker build -t svc-6-dashboard .
}

upload_all(){
  local s
  local services
  local image_id
  echo "Uploading all services..."
  services=(
    "svc-collector"
    "svc-ingestor"
    "svc-local-storage"
    "svc-processor"
    "svc-central-storage"
    "svc-dashboard"
  )
  for s in "${services[@]}"; do
    image_id=$(docker images -q $s)
    docker_upload ${image_id} $s "scinet"
    docker_upload ${image_id} $s "vaughan"
  done
}


if [[ $build -eq 1 ]]; then
  build_all
fi

if [[ $upload -eq 1 ]]; then
  upload_all
fi

