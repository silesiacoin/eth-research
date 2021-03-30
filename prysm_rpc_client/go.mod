module github.com/atif-konasl/eth-research/prysm_rpc_client

go 1.16

require (
	github.com/ethereum/go-ethereum v1.10.1
	github.com/gogo/protobuf v1.3.1
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/pkg/errors v0.8.1
	github.com/prysmaticlabs/eth2-types v0.0.0-20210127031309-22cbe426eba6
	github.com/prysmaticlabs/ethereumapis v0.0.0-20210311175904-cf9f64632dd4
	github.com/sirupsen/logrus v1.6.0
	google.golang.org/grpc v1.33.1
)

replace github.com/prysmaticlabs/ethereumapis => github.com/lukso-network/vanguard-apis v0.0.0-20210326133834-299effe02137
