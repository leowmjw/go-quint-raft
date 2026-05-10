module github.com/leowmjw/go-quint-raft/examples/raft

go 1.26.1

replace github.com/leowmjw/go-quint-raft/quint-connector-go => ../../quint-connector-go

require (
	github.com/ani03sha/raftly v0.2.0
	github.com/informalsystems/itf-go v0.0.1
	github.com/leowmjw/go-quint-raft/quint-connector-go v0.0.0-00010101000000-000000000000
)

require (
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260120221211-b8f7ae30c516 // indirect
	google.golang.org/grpc v1.80.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
