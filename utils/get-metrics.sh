#!/bin/bash

HOSTNAME="localhost"
PORT=(8001 8002 8003 8004 8005 8006)
for port in "${PORT[@]}"; do
  case $port in
    8001)
      service="ingestor"
      ;;
    8002)
      service="local-storage"
      ;;
    8003)
      service="processor"
      ;;
    8004)
      service="central-storage"
      ;;
    8005)
      service=""
      ;;
    8006)
      service="central-storage"
      ;;
    *)
      echo "Unknown port: $port"
      continue
      ;;
  esac

  # Get processing metrics
  ./collect-metrics.sh --host $HOSTNAME --port $port -p -t mean

  # Get RTT metrics
  if [[ -z $service ]]; then
    echo "skip"
  else
    ./collect-metrics.sh --host ${HOSTNAME} --port ${port} --rtt -t "mean" --service "$service"
  fi

  # Get the list of services and types
  ./collect-metrics.sh --host ${HOSTNAME} --port ${port} --index
  
done
