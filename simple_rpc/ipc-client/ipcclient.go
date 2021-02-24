package ipc_client

import (
	"github.com/atif-konasl/eth-research/simple_rpc/api"
	"github.com/atif-konasl/eth-research/simple_rpc/rpc"
)

type IpcClient struct {
	isConnected 	        bool
	isRunning               bool
	endpoint				string
	client					*rpc.Client
}


func NewIpcClilent(endpoint string) (*IpcClient, error) {
	client, err := rpc.Dial(endpoint)
	if err != nil {
		return nil, err
	}

	return &IpcClient{
		isConnected: true,
		isRunning: true,
		endpoint: endpoint,
		client: client,
	}, nil
}

func (ipcClient *IpcClient) ProduceCatalystBlock(data api.ExtraData) (*api.ShardInfo, error) {
	var shardInfo api.ShardInfo
	if err := ipcClient.client.Call(&shardInfo, "orchestrator_produceCatalystBlock", data); err != nil {
		return nil, err
	}
	return &shardInfo, nil
}