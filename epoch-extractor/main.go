package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
)

var dialInterval = 10 * time.Second
var errConnectionIssue = errors.New("could not connect")

var (
	timeNow             = time.Now().Unix()
	vanguardRPCEndpoint = flag.String(
		"vanguard-rpc-endpoint",
		"127.0.0.1:4000",
		"Vanguard node RPC provider endpoint(Default: 127.0.0.1:4000",
	)
	slotsPerEpoch = flag.Uint64(
		"slots-per-epoch",
		32,
		"Number of slots per epoch(Default: 32",
	)
	pandoraRPCEndpoint = flag.String(
		"pandora-rpc-endpoint",
		"127.0.0.1:8545",
		"Pandora node RP provider endpoint(Default: 127.0.0.1:8545",
	)
	genesisTimeStart = flag.Int64(
		"genesis-time-start",
		timeNow,
		fmt.Sprintf("Genesis time start(Default: %d (now)", timeNow),
	)
)

// Client provides proposer list for current epoch as well as next epoch
type Client struct {
	assignments            *ethpb.ValidatorAssignments
	curSlot                types.Slot
	curEpoch               types.Epoch
	prevEpoch              types.Epoch
	curEpochSlotToProposer map[types.Slot]string //
	conn                   *grpc.ClientConn
	pandoraConn            *rpc.Client
	genesisTime            uint64
	grpcRetryDelay         time.Duration
	grpcRetries            uint
	maxCallRecvMsgSize     int
	cancel                 context.CancelFunc
	ctx                    context.Context
	endpoint               string
	beaconClient           ethpb.BeaconChainClient
	validatorClient        ethpb.BeaconNodeValidatorClient
	SlotsPerEpoch          types.Slot
}

func NewClient(
	ctx context.Context,
	vanguardRPCEndpoint string,
	slotsPerEpoch uint64,
	pandoraRPCEndpoint string,
) *Client {
	ctx, cancel := context.WithCancel(ctx)
	client, err := rpc.Dial(pandoraRPCEndpoint)

	if nil != err {
		panic(err.Error())
	}

	return &Client{
		curSlot:                0,
		curEpoch:               0,
		prevEpoch:              0,
		curEpochSlotToProposer: make(map[types.Slot]string, 32),
		grpcRetries:            5,
		grpcRetryDelay:         dialInterval,
		maxCallRecvMsgSize:     4194304, // 4mb
		cancel:                 cancel,
		pandoraConn:            client,
		ctx:                    ctx,
		endpoint:               vanguardRPCEndpoint,
		SlotsPerEpoch:          types.Slot(slotsPerEpoch),
	}
}

func main() {
	flag.Parse()

	if *slotsPerEpoch == 0 {
		log.Print("No --slots-per-epoch specified, defaulting 32")
	}

	if *vanguardRPCEndpoint == "" {
		log.Print("No --vanguard-rpc-endpoint specified, defaulting 127.0.0.1:4000")
	}

	if "" == *pandoraRPCEndpoint {
		log.Print("No --pandora-rpc-endpoint specified, defaulting 127.0.0.1:8545")
	}

	if timeNow == *genesisTimeStart {
		log.Printf("No --genesis-time-start specified, defaulting %d", timeNow)
	}

	ctx := context.Background()
	c := NewClient(ctx, *vanguardRPCEndpoint, *slotsPerEpoch, *pandoraRPCEndpoint)
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
	c.genesisTime = uint64(*genesisTimeStart)

	// start client runner
	c.runner()
}

func (c *Client) runner() {
	log.Info("Starting epoch explorer")
	ticker := time.NewTicker(dialInterval)
	defer ticker.Stop()
	firstTime := true
	rpcClient := c.pandoraConn
	epochDuration := time.Duration(6) * time.Duration(32)

	for {
		select {
		case <-ticker.C:
			if err := c.CanonicalHeadSlot(); err == nil {
				curEpoch := types.Epoch(c.curSlot.DivSlot(c.SlotsPerEpoch))
				c.curEpoch = curEpoch
				log.WithField("curEpoch", curEpoch).WithField("curSlot", c.curSlot).Info("canonical head info")

				// epoch changed so get next epoch proposer list
				if firstTime || c.curEpoch >= c.prevEpoch+1 {
					c.prevEpoch = c.curEpoch
					// getting proposer list for next epoch(curEpoch + 1)
					nextAssignments, err := c.NextEpochProposerList()
					if err != nil {
						log.WithError(err).Error("got error from NextEpochProposerList api")
						return
					}

					firstTime = false

					notifyPandoraFunc := func() {
						// Here we notify Pandora about epoch
						var response bool
						validatorsListPayload := make([]string, 0)

						// Add additional field at start for genesis purpose
						if 0 == nextAssignments.Epoch && len(nextAssignments.Assignments) > 32 {
							validatorsListPayload = append(validatorsListPayload, "0x")
						}

						for index, bytesValidator := range nextAssignments.Assignments {
							validator := hexutil.Encode(bytesValidator.PublicKey)

							if len(validatorsListPayload) <= 32 {
								validatorsListPayload = append(validatorsListPayload, validator)
							}

							if len(validatorsListPayload) > 32 {
								log.WithField("validators", validator).WithField("index", index).Warn("Invalid index")
								break
							}
						}

						currentEpochStart := c.genesisTime

						if c.curEpoch > 0 {
							currentEpochStart = currentEpochStart + (uint64(c.curEpoch) * uint64(epochDuration))
						}

						log.WithField("validatorsLen", len(validatorsListPayload)).Info(
							"eth_insertMinimalConsensusInfo")

						err = rpcClient.Call(
							&response,
							"eth_insertMinimalConsensusInfo",
							uint64(c.curEpoch),
							validatorsListPayload,
							currentEpochStart,
						)

						if nil != err {
							log.WithError(err).Error("got error when filling minimal consensus info")

							return
						}

						msg := "succeed to fill minimal consensus info"

						if !response {
							msg = fmt.Sprintf("did not %s", msg)
						}

						log.WithField("validatorsLen", len(validatorsListPayload)).Info(msg)
					}

					notifyPandoraFunc()
				}
			}
		case <-c.ctx.Done():
			log.Debug("Stopping grpc committee fetcher service....")
			return
		}
	}
}

func (c *Client) NextEpochProposerList() (*ethpb.ValidatorAssignments, error) {
	assignments, err := c.beaconClient.NextEpochProposerList(c.ctx, &ptypes.Empty{})
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
