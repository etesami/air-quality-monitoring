#!/bin/bash

while [[ "$#" -gt 0 ]]; do
    case $1 in
        --host) host="$2"; shift ;;
        --port) port="$2"; shift ;;
        --service) service="$2"; shift ;;
        -p|--processing) processing=1 ;;
        -r|--rtt) rtt=1 ;;
        -t|--type) type="$2"; shift ;;
        --index) index=1 ;;
        -h|--help) help=1 ;;
        *) echo "Unknown parameter passed: $1"; exit 1 ;;
    esac
    shift
done

if [[ $help ]]; then
    echo "Usage: $0 [options]"
    echo "Options:"
    echo "  -p, --processing    Collect processing metrics"
    echo "  -r, --rtt           Collect RTT metrics"
    echo "  -t, --type   Type of metric (e.g., mean, max, min)"
    echo "  --service           Service name"
    echo "  --host              Hostname"
    echo "  --port              Port number"
    echo "  --index             List models"
    echo "  -h, --help          Show this help message"
    exit 0
fi

if [[ -z $host ]]; then
    echo "Error: Host is required."
    exit 1
fi
if [[ -z $port ]]; then
    echo "Error: Port is required."
    exit 1
fi


call_index(){
  local hostname=$1
  local port=$2
  local query="${hostname}:${port}/metrics"
  echo "Query: ${query}"
  curl -s ${query} 
}

get_processing() {
  local HOSTNAME=$1
  local PORT=$2
  local SERVICES=$3
  local METRIC_TYPES=$4
  local service
  # local METRIC_TYPES=("mean" "max" "min")
  for service in "${SERVICES[@]}"; do
    for metric_type in "${METRIC_TYPES[@]}"; do
      local query="${HOSTNAME}:${PORT}/metrics/processing?type=${METRIC_TYPES}&service=processing"
      echo "Query: ${query}"
      curl -s ${query} | jq -r
    done
  done
}

get_rtt() {
  local HOSTNAME=$1
  local PORT=$2
  local SERVICES=$3
  local METRIC_TYPES=$4
  local service
  # local METRIC_TYPES=("mean" "max" "min")
  for service in "${SERVICES[@]}"; do
    for metric_type in "${METRIC_TYPES[@]}"; do
      local query="${HOSTNAME}:${PORT}/metrics/rtt?type=${METRIC_TYPES}&service=${service}"
      echo "Query: ${query}"
      curl -s ${query} | jq -r
    done
  done
}

if [[ $index -eq 1 ]]; then 
  # Get the list of models
  call_index ${host} ${port} "index"
  exit 0
fi

if [[ $processing -eq 1 ]]; then
  if [[ -z $type ]]; then
    echo "Error: Type is required for processing metrics."
    exit 1
  fi
  # Get the processing metrics
  get_processing ${host} ${port} "processing" $type
fi

if [[ $rtt -eq 1 ]]; then
  if [[ -z $type ]]; then
    echo "Error: Type is required for RTT metrics."
    exit 1
  fi
  if [[ -z $service ]]; then
    echo "Error: Service is required for RTT metrics."
    exit 1
  fi
  # Get the RTT metrics
  get_rtt ${host} ${port} "$service" "$type"
fi