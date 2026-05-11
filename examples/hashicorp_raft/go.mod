module github.com/leowmjw/go-quint-raft/examples/hashicorp_raft

go 1.24.0

replace github.com/leowmjw/go-quint-raft/quint-connector-go => ../../quint-connector-go

require (
	github.com/hashicorp/raft v1.7.3
	github.com/informalsystems/itf-go v0.0.1
	github.com/leowmjw/go-quint-raft/quint-connector-go v0.0.0-00010101000000-000000000000
)

require (
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/fatih/color v1.13.0 // indirect
	github.com/hashicorp/go-hclog v1.6.2 // indirect
	github.com/hashicorp/go-immutable-radix v1.0.0 // indirect
	github.com/hashicorp/go-metrics v0.5.4 // indirect
	github.com/hashicorp/go-msgpack/v2 v2.1.2 // indirect
	github.com/hashicorp/golang-lru v0.5.0 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	golang.org/x/sys v0.40.0 // indirect
)
