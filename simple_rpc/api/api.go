package api

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"
)

// ExternalAPI defines the external API through which signing requests are made.
type ExternalAPI interface {
	// List available accounts
	ProduceCatalystBlock(ctx context.Context, data ExtraData) (*ShardInfo, error)

	GetShardInfo(ctx context.Context) (*ShardInfo, error)
}


type OrchestratorApi struct {
	version 	uint64
	name 		string
}

func NewOrchestratorApi(version uint64, name string) *OrchestratorApi {
	return &OrchestratorApi{
		version: version,
		name: name,
	}
}

func (orcApi *OrchestratorApi) ProduceCatalystBlock(ctx context.Context, data ExtraData) (*ShardInfo, error) {
	log.Info("Got a request for catalyst block production", "extraData", data)
	return &ShardInfo{
		ParentHash: common.Hash{},
		Coinbase: common.Address{},
		Root: common.Hash{},
		TxHash: common.Hash{},
		ReceiptHash: common.Hash{},
		Number: big.NewInt(45),
	}, nil
}


func (orcApi *OrchestratorApi) GetShardInfo(ctx context.Context) (*ShardInfo, error) {
	log.Info("Got a request for shard info")
	return &ShardInfo{
		ParentHash: common.Hash{},
		Coinbase: common.Address{},
		Root: common.Hash{},
		TxHash: common.Hash{},
		ReceiptHash: common.Hash{},
		Number: big.NewInt(45),
	}, nil
}