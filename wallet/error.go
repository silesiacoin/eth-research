package wallet

import "errors"

// ErrZeroKey describes an error due to a zero secret key.
var ErrZeroKey = errors.New("received secret key is zero")

// ErrSecretUnmarshal describes an error which happens during unmarshalling
// a secret key.
var ErrSecretUnmarshal = errors.New("could not unmarshal bytes into secret key")

// ErrInfinitePubKey describes an error due to an infinite public key.
var ErrInfinitePubKey = errors.New("received an infinite public key")

var ErrNoWalletFound = errors.New(
"no wallet found. You can create a new wallet with `validator wallet create`. " +
"If you already did, perhaps you created a wallet in a custom directory, which you can specify using " +
"`--wallet-dir=/path/to/my/wallet`",
)

// ErrSigFailedToVerify returns when a signature of a block object(ie attestation, slashing, exit... etc)
// failed to verify.
var ErrSigFailedToVerify = errors.New("signature did not verify")