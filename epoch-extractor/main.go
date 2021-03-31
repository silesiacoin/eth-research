package main

import (
	"context"
	"flag"
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

var (
	vanguardRPCEndpoint = flag.String(
		"vangurad-rpc-endpoint",
		"127.0.0.1:4000",
		"Vanguard node RPC provider endpoint(Default: 127.0.0.1:4000",
		)
	slotsPerEpoch = flag.Uint64(
		"slots-per-epoch",
		32,
		"Number of slots per epoch(Default: 32",
		)
)

// Client provides proposer list for current epoch as well as next epoch
type Client struct {
	assignments           			*ethpb.ValidatorAssignments
	curSlot			      			types.Slot
	curEpoch						types.Epoch
	prevEpoch						types.Epoch
	nextEpochProposerIndexToPubKey  map[types.ValidatorIndex]string		// validator index to public key mapping for current epoch
	nextEpochSlotToProposerIndex  	map[types.Slot]types.ValidatorIndex //
	curEpochProposerIndexToPubKey   map[types.ValidatorIndex]string		// validator index to public key mapping for current epoch
	curEpochSlotToProposerIndex  	map[types.Slot]types.ValidatorIndex //
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

func NewClient(ctx context.Context, vanguardRPCEndpoint string, slotsPerEpoch uint64) (*Client) {
	ctx, cancel := context.WithCancel(ctx)

	return &Client{
		curSlot: 0,
		curEpoch: 0,
		prevEpoch: 0,
		nextEpochProposerIndexToPubKey: make(map[types.ValidatorIndex]string, 32),
		nextEpochSlotToProposerIndex: make(map[types.Slot]types.ValidatorIndex, 32),
		curEpochProposerIndexToPubKey: make(map[types.ValidatorIndex]string, 32),
		curEpochSlotToProposerIndex: make(map[types.Slot]types.ValidatorIndex, 32),
		grpcRetries: 5,
		grpcRetryDelay: dialInterval,
		maxCallRecvMsgSize: 4194304, 	// 4mb
		cancel: cancel,
		ctx: ctx,
		endpoint: vanguardRPCEndpoint,
		SlotsPerEpoch: types.Slot(slotsPerEpoch),
	}
}

func main() {
	flag.Parse()

	if *slotsPerEpoch == 0 {
		log.Print("No --slots-per-epoch specified, defaulting 32")
	}

	if *vanguardRPCEndpoint == "" {
		log.Print("No --vangurad-rpc-endpoint specified, defaulting 127.0.0.1:4000")
	}

	ctx := context.Background()
	c := NewClient(ctx, *vanguardRPCEndpoint, *slotsPerEpoch)
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
	log.Info("Starting epoch explorer")
	ticker := time.NewTicker(dialInterval)
	defer ticker.Stop()
	firsTime := true

	for {
		select {
		case <-ticker.C:
			if err:= c.CanonicalHeadSlot(); err == nil {
				curEpoch := types.Epoch(c.curSlot.DivSlot(c.SlotsPerEpoch))
				c.curEpoch = curEpoch
				log.WithField("curEpoch", curEpoch).WithField("curSlot", c.curSlot).Info("canonical head info")

				// epoch changed so get next epoch proposer list
				if firsTime || c.curEpoch >= c.prevEpoch + 1 {
					c.prevEpoch = c.curEpoch
					// getting proposer list for next epoch(curEpoch + 1)
					nextAssignments, err := c.NextEpochProposerList()
					if err != nil {
						log.WithError(err).Error("got error from NextEpochProposerList api")
						return
					}
					proposerIndexToPubKey, slotToProposerIndex := c.processNextEpochAssignments(nextAssignments)
					// Store current
					c.curEpochProposerIndexToPubKey = c.nextEpochProposerIndexToPubKey
					c.curEpochSlotToProposerIndex = c.nextEpochSlotToProposerIndex
					// Store next
					c.nextEpochProposerIndexToPubKey = proposerIndexToPubKey
					c.nextEpochSlotToProposerIndex = slotToProposerIndex

					// getting proposer list for current epoch
					curAssignments, err := c.GetListValidatorAssignments(c.curEpoch)
					if err != nil {
						log.WithError(err).Error("got error when getting validator assignments info")
						return
					}

					// For the first epoch only
					if firsTime {
						firsTime = false
						proposerIndexToPubKey, slotToProposerIndex := c.processNextEpochAssignments(curAssignments)
						//c.logProposerSchedule(c.curEpoch, proposerIndexToPubKey, slotToProposerIndex)
						c.curEpochProposerIndexToPubKey = proposerIndexToPubKey
						c.curEpochSlotToProposerIndex = slotToProposerIndex
					}

					c.logProposerSchedule(c.curEpoch, c.curEpochProposerIndexToPubKey, c.curEpochSlotToProposerIndex)
					c.logProposerSchedule(c.curEpoch + 1, c.nextEpochProposerIndexToPubKey, c.nextEpochSlotToProposerIndex)
					c.checkCompatibility(curAssignments)
				}
			}
		case <-c.ctx.Done():
			log.Debug("Stopping grpc committee fetcher service....")
			return
		}
	}
}

func (c *Client) logProposerSchedule(
	epoch types.Epoch, proposerIndexToPubKey map[types.ValidatorIndex]string,
	 slotToProposerIndex map[types.Slot]types.ValidatorIndex) {

	log.WithField("epoch", epoch).Info("Showing epoch info...")
	// To store the keys in slice in sorted order
	keys := make([]int, len(slotToProposerIndex))
	i := 0
	for k := range slotToProposerIndex {
		keys[i] = int(uint64(k))
		i++
	}
	sort.Ints(keys)

	// To perform the opertion you want
	for _, k := range keys {
		slot := types.Slot(uint64(k))
		proposerIndex := slotToProposerIndex[slot]
		log.WithField("slot", slot).WithField(
			"nextEpoch", epoch).WithField(
				"proposerPubKey", "0x" + proposerIndexToPubKey[proposerIndex]).Info(" Proposer schedule")
	}
}

func (c *Client) checkCompatibility(curAssignments *ethpb.ValidatorAssignments)  {
	for _, assignment := range curAssignments.Assignments {
		for _, slot := range assignment.ProposerSlots {
			if len(c.nextEpochProposerIndexToPubKey[c.nextEpochSlotToProposerIndex[slot]]) > 0 &&
				c.curEpochSlotToProposerIndex[slot] != assignment.ValidatorIndex {

				log.WithField("slot", slot).WithField(
					"curEpoch", c.curEpoch).WithField(
						"giveProposerPubKey", "0x" + common.Bytes2Hex(assignment.PublicKey)[:12]).WithField(
							"storedProposerPubKey", "0x" + c.nextEpochProposerIndexToPubKey[c.nextEpochSlotToProposerIndex[slot]][:12]).Error("Proposer public not matched!")
				return
			}
		}
	}
}

func (c *Client) processNextEpochAssignments(assignments *ethpb.ValidatorAssignments) (
	map[types.ValidatorIndex]string, map[types.Slot]types.ValidatorIndex) {

	proposerIndexToPubKey := make(map[types.ValidatorIndex]string, c.SlotsPerEpoch)
	slotToProposerIndex := make(map[types.Slot]types.ValidatorIndex, c.SlotsPerEpoch)

	for _, assignment := range assignments.Assignments {
		for _, proposerSlot := range assignment.ProposerSlots {
			slotToProposerIndex[proposerSlot] = assignment.ValidatorIndex
		}
		// this validator is a proposer
		if len(assignment.ProposerSlots) > 0 {
			proposerIndexToPubKey[assignment.ValidatorIndex] = common.Bytes2Hex(assignment.PublicKey)
		}
	}

	return proposerIndexToPubKey, slotToProposerIndex
}


func (c *Client) NextEpochProposerList() (*ethpb.ValidatorAssignments, error) {
	assignments, err := c.beaconClient.NextEpochProposerList(c.ctx, &ptypes.Empty{})
	return assignments, err
}

func (c *Client) GetListValidatorAssignments(epoch types.Epoch) (*ethpb.ValidatorAssignments, error) {
	assignments, err := c.beaconClient.ListValidatorAssignments(c.ctx, &ethpb.ListValidatorAssignmentsRequest{
		QueryFilter: &ethpb.ListValidatorAssignmentsRequest_Epoch{
			Epoch: epoch,
		},
	})
	return assignments, err
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
