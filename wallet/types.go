package wallet

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/sha3"
	"io"
	"sync"
)

type SlotInfo struct {
	Epoch 				 uint64
	Slot                 uint64
	ProposerIndex        uint64
}

func NewSlotInfo(epoch, slot, proposerIndex uint64) *SlotInfo {
	return &SlotInfo{
		Epoch: epoch,
		Slot: slot,
		ProposerIndex: proposerIndex,
	}
}

// hasherPool holds LegacyKeccak hashers.
var hasherPool = sync.Pool{
	New: func() interface{} {
		return sha3.NewLegacyKeccak256()
	},
}

func rlpHash(x interface{}) (h common.Hash) {
	sha := hasherPool.Get().(crypto.KeccakState)
	defer hasherPool.Put(sha)
	sha.Reset()
	rlp.Encode(sha, x)
	sha.Read(h[:])
	return h
}

// Hash returns the block hash of the header, which is simply the keccak256 hash of its
// RLP encoding.
func (s *SlotInfo) Hash() common.Hash {
	return rlpHash(s)
}

// DecodeRLP decodes the Ethereum
func (s *SlotInfo) DecodeRLP(rlpData *rlp.Stream) error {
	var eb SlotInfo
	if err := rlpData.Decode(&eb); err != nil {
		return err
	}
	s.Epoch, s.Slot, s.ProposerIndex = eb.Epoch, eb.Slot, eb.ProposerIndex
	return nil
}

// EncodeRLP serializes b into the Ethereum RLP block format.
func (s *SlotInfo) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, SlotInfo{
		Epoch:  		s.Epoch,
		Slot:    		s.Slot,
		ProposerIndex: 	s.ProposerIndex,
	})
}
