#!/bin/bash

# We want to use some private functinos defined here
source /home/ubuntu/.functions.sh

while [[ "$#" -gt 0 ]]; do
  case $1 in
    -v|--version) version="$2"; shift ;;
    -s|--service) svc_name="$2"; shift ;;
    -d|--svc-dir) svc_dir="$2"; shift ;;
    -b|--build) build=1 ;;
    -u|--upload) upload=1 ;;
    *) echo "Unknown parameter passed: $1"; exit 1 ;;
  esac
  shift
done

if [[ -z $version ]]; then
  echo "Version not specified. Exiting..."
  exit 1
fi
if [[ ! -z $svc_name ]]; then
  if [[ -z $svc_dir ]]; then
    echo "Service directory not specified. Exiting..."
    exit 1
  fi
fi


build_service() {
  local sPath=$1
  local sName=$2
  local v=$version
  echo "Building service $sName..."
  PARENT_DIR=$(dirname "$(realpath "$0")")

  cd $PARENT_DIR/../$sPath
  docker build -t $sName:$v .
}

build_all() {
  local v=$version
  echo "Building all services..."
  PARENT_DIR=$(dirname "$(realpath "$0")")

  cd $PARENT_DIR/../svc-1-data-collector
  docker build -t svc-collector:$v .

  cd $PARENT_DIR/../svc-2-data-ingestor
  docker build -t svc-ingestor:$v .

  cd $PARENT_DIR/../svc-3-local-storage
  docker build -t svc-local-storage:$v .

  cd $PARENT_DIR/../svc-4-data-processor
  docker build -t svc-processor:$v .

  cd $PARENT_DIR/../svc-5-central-storage
  docker build -t svc-central-storage:$v .

  cd $PARENT_DIR/../svc-6-dashboard
  docker build -t svc-dashboard:$v .
}

upload_all(){
  local s
  local services
  local image_id
  local v=$version
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
    image_id=$(docker images -q $s:$v)
    docker_upload ${image_id} $s:$v "scinet"
    docker_upload ${image_id} $s:$v "vaughan"
  done
}

upload_service(){
  local s=$1
  local image_id
  local v=$version
  echo "Uploading service $s..."

  image_id=$(docker images -q $s:$v)
  docker_upload ${image_id} $s:$v "scinet"
  docker_upload ${image_id} $s:$v "vaughan"
}


if [[ $build -eq 1 ]]; then
  if [[ -z $svc_name ]]; then
    build_all
  else
    build_service $svc_dir $svc_name
  fi
fi

if [[ $upload -eq 1 ]]; then
  if [[ -z $svc_name ]]; then
    upload_all
  else
    upload_service $svc_name
  fi
fi

