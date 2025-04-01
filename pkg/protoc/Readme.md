# Proto Readme

```bash

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

protoc \
  --go_out=. \
  --go_opt=paths=source_relative \
  --go-grpc_out=. \
  --go-grpc_opt=paths=source_relative air_quality_monitoring.proto
```