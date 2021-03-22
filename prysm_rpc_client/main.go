package main

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"sort"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
)

var dialInterval = 5 * time.Second
var errConnectionIssue = errors.New("could not connect")

// Client provides proposer list for current epoch as well as next epoch
type Client struct {
	assignments           			*ethpb.ValidatorAssignments
	curSlot			      			types.Slot
	curEpoch						types.Epoch
	proposerIndexToPubKey    	  	map[types.ValidatorIndex]string		// validator index to public key mapping for current epoch
	slotToProposerIndex  			map[types.Slot]types.ValidatorIndex //
	conn                  			*grpc.ClientConn
	grpcRetryDelay        			time.Duration
	grpcRetries           			uint
	maxCallRecvMsgSize    			int
	cancel                			context.CancelFunc
	ctx 				  			context.Context
	endpoint              			string
	beaconClient	      			ethpb.BeaconChainClient
	validatorClient       			ethpb.BeaconNodeValidatorClient
	SlotsPerEpoch         			types.Slot
}

func NewClient(ctx context.Context) (*Client) {
	ctx, cancel := context.WithCancel(ctx)

	return &Client{
		curSlot: 0,
		curEpoch: 0,
		proposerIndexToPubKey: make(map[types.ValidatorIndex]string, 32),
		slotToProposerIndex : make(map[types.Slot]types.ValidatorIndex, 32),
		grpcRetries: 5,
		grpcRetryDelay: dialInterval,
		maxCallRecvMsgSize: 4194304, 	// 4mb
		cancel: cancel,
		ctx: ctx,
		endpoint: "34.91.111.241:4000",
		SlotsPerEpoch: 32,
	}
}

func main() {

	ctx := context.Background()
	c := NewClient(ctx)
	dialOpts := ConstructDialOptions(
		c.maxCallRecvMsgSize,
		"",
		c.grpcRetries,
		c.grpcRetryDelay,
	)
	if dialOpts == nil {
		return
	}

	conn, err := grpc.DialContext(ctx, c.endpoint, dialOpts...)
	if err != nil {
		log.Errorf("Could not dial endpoint: %s, %v", c.endpoint, err)
		return
	}
	defer conn.Close()
	c.conn = conn
	c.validatorClient = ethpb.NewBeaconNodeValidatorClient(conn)
	c.beaconClient = ethpb.NewBeaconChainClient(conn)

	// start client runner
	c.runner()
}

func (c *Client) runner() {
	log.Info("Starting proposer info finder...")
	ticker := time.NewTicker(dialInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err:= c.CanonicalHeadSlot(); err == nil {
				if c.curSlot % c.SlotsPerEpoch == 0 {
					c.curEpoch = types.Epoch(c.curSlot.DivSlot(c.SlotsPerEpoch))
					curAssignments, err := c.GetListValidatorAssignments(c.curEpoch)
					if err == nil {
						c.assignments = curAssignments
						c.processAssignments(c.curSlot, curAssignments)
						c.logProposerInfo()
					}
				}
			}
		case <-c.ctx.Done():
			log.Debug("Stopping grpc committee fetcher service....")
			return
		}
	}
}

func (c *Client) logProposerInfo() {
	log.WithField("epoch", c.curEpoch).Info("current epoch")
	// To store the keys in slice in sorted order
	keys := make([]int, len(c.slotToProposerIndex))
	i := 0
	for k := range c.slotToProposerIndex {
		keys[i] = int(uint64(k))
		i++
	}
	sort.Ints(keys)

	// To perform the opertion you want
	for _, k := range keys {
		slot := types.Slot(uint64(k))
		proposerIndex := c.slotToProposerIndex[slot]
		log.WithField("slot", slot).WithField("proposerIndex", proposerIndex).WithField(
			"proposerPublicKey", c.proposerIndexToPubKey[proposerIndex]).Info("proposer info")
	}
}


func (c *Client) processAssignments(slot types.Slot, assignments *ethpb.ValidatorAssignments) {
	proposerIndexToPubKey := make(map[types.ValidatorIndex]string, c.SlotsPerEpoch)
	slotToProposerIndex := make(map[types.Slot]types.ValidatorIndex, c.SlotsPerEpoch)
	//slotOffset := slot - (slot % c.SlotsPerEpoch)

	for _, assignment := range assignments.Assignments {
		for _, proposerSlot := range assignment.ProposerSlots {
			//proposerIndex := proposerSlot - slotOffset
			//if proposerIndex >= c.SlotsPerEpoch {
			//	log.WithField("assignment", assignment).Warn("Invalid proposer slot")
			//}
			//log.WithField("proposerSlot", proposerSlot).WithField(
			//	"proposerIndex", assignment.ValidatorIndex).Info(
			//		"got validator index and proposer slots of this validator")
			slotToProposerIndex[proposerSlot] = assignment.ValidatorIndex
		}
		// this validator is a proposer
		if len(assignment.ProposerSlots) > 0 {
			proposerIndexToPubKey[assignment.ValidatorIndex] = common.Bytes2Hex(assignment.PublicKey)
		}
	}
	c.proposerIndexToPubKey = proposerIndexToPubKey
	c.slotToProposerIndex = slotToProposerIndex
}


func (c *Client) GetListValidatorAssignments(epoch types.Epoch) (*ethpb.ValidatorAssignments, error) {
	assignments, err := c.beaconClient.ListValidatorAssignments(c.ctx, &ethpb.ListValidatorAssignmentsRequest{
		QueryFilter: &ethpb.ListValidatorAssignmentsRequest_Epoch{
			Epoch: epoch,
		},
	})
	if err != nil {
		log.WithError(err).Error("got error when getting validator assignments info")
		return nil, err
	}
	//log.WithField("requestedEpoch", epoch).WithField(
	//	"responseEpoch", assignments.Epoch).Info("successfully got validator assignments")
	return assignments, nil
}

// CanonicalHeadSlot returns the slot of canonical block currently found in the
// beacon chain via RPC.
func (c *Client) CanonicalHeadSlot() error {
	head, err := c.beaconClient.GetChainHead(c.ctx, &ptypes.Empty{})
	if err != nil {
		log.WithError(err).Info("failed to get canonical head")
		return err
	}
	c.curSlot = head.HeadSlot
		//types.Epoch(head.HeadSlot / c.SlotsPerEpoch)
	return nil
}

// ConstructDialOptions constructs a list of grpc dial options
func ConstructDialOptions(
	maxCallRecvMsgSize int,
	withCert string,
	grpcRetries uint,
	grpcRetryDelay time.Duration,
	extraOpts ...grpc.DialOption,
) []grpc.DialOption {
	var transportSecurity grpc.DialOption
	if withCert != "" {
		creds, err := credentials.NewClientTLSFromFile(withCert, "")
		if err != nil {
			log.Errorf("Could not get valid credentials: %v", err)
			return nil
		}
		transportSecurity = grpc.WithTransportCredentials(creds)
	} else {
		transportSecurity = grpc.WithInsecure()
		log.Warn("You are using an insecure gRPC connection. If you are running your beacon node and " +
			"validator on the same machines, you can ignore this message. If you want to know " +
			"how to enable secure connections, see: https://docs.prylabs.network/docs/prysm-usage/secure-grpc")
	}

	if maxCallRecvMsgSize == 0 {
		maxCallRecvMsgSize = 10 * 5 << 20 // Default 50Mb
	}

	dialOpts := []grpc.DialOption{
		transportSecurity,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxCallRecvMsgSize),
			grpc_retry.WithMax(grpcRetries),
			grpc_retry.WithBackoff(grpc_retry.BackoffLinear(grpcRetryDelay)),
		),
	}

	dialOpts = append(dialOpts, extraOpts...)
	return dialOpts
}
