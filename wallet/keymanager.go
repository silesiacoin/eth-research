package wallet

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"

	"github.com/atif-konasl/eth-research/bls"
	"github.com/atif-konasl/eth-research/bls/herumi"
	"github.com/atif-konasl/eth-research/bytesutil"
	"github.com/pkg/errors"
	keystorev4 "github.com/wealdtech/go-eth2-wallet-encryptor-keystorev4"
)

var (
	lock              sync.RWMutex
	orderedPublicKeys = make([][48]byte, 0)
	secretKeysCache   = make(map[[48]byte]bls.SecretKey)
)

const (
	// KeystoreFileNameFormat exposes the filename the keystore should be formatted in.
	KeystoreFileNameFormat = "keystore-%d.json"
	// AccountsPath where all imported keymanager keystores are kept.
	AccountsPath = "accounts"
	// AccountsKeystoreFileName exposes the name of the keystore file.
	AccountsKeystoreFileName = "all-accounts.keystore.json"
)

// Defines a struct containing 1-to-1 corresponding
// private keys and public keys for eth2 validators.
type accountStore struct {
	PrivateKeys [][]byte `json:"private_keys"`
	PublicKeys  [][]byte `json:"public_keys"`
}


// Keymanager implementation for imported keystores utilizing EIP-2335.
type Keymanager struct {
	wallet              *Wallet
	accountsStore       *accountStore
	disabledPublicKeys  map[[48]byte]bool
}


// AccountsKeystoreRepresentation defines an internal Prysm representation
// of validator accounts, encrypted according to the EIP-2334 standard
// but containing extra fields such as markers for disabled public keys.
type AccountsKeystoreRepresentation struct {
	Crypto             map[string]interface{} `json:"crypto"`
	ID                 string                 `json:"uuid"`
	Version            uint                   `json:"version"`
	Name               string                 `json:"name"`
	DisabledPublicKeys []string               `json:"disabled_public_keys"`
}

// NewKeymanager instantiates a new imported keymanager from configuration options.
func NewKeymanager(wallet *Wallet) (*Keymanager, error) {
	k := &Keymanager{
		wallet:              wallet,
		accountsStore:       &accountStore{},
		disabledPublicKeys:  make(map[[48]byte]bool),
	}

	if err := k.initializeAccountKeystore(); err != nil {
		return nil, errors.Wrap(err, "failed to initialize account store")
	}

	return k, nil
}

func  (km *Keymanager) initializeAccountKeystore() error {
	encoded, err := km.wallet.ReadFileAtPath(AccountsPath, AccountsKeystoreFileName)
	if err != nil && strings.Contains(err.Error(), "no files found") {
		// If there are no keys to initialize at all, just exit.
		return nil
	} else if err != nil {
		return errors.Wrapf(err, "could not read keystore file for accounts %s", AccountsKeystoreFileName)
	}
	keystoreFile := &AccountsKeystoreRepresentation{}
	if err := json.Unmarshal(encoded, keystoreFile); err != nil {
		return errors.Wrapf(err, "could not decode keystore file for accounts %s", AccountsKeystoreFileName)
	}
	// We extract the validator signing private key from the keystore
	// by utilizing the password and initialize a new BLS secret key from
	// its raw bytes.
	password := km.wallet.walletPassword
	decryptor := keystorev4.New()
	enc, err := decryptor.Decrypt(keystoreFile.Crypto, password)
	if err != nil && strings.Contains(err.Error(), "invalid checksum") {
		return errors.Wrap(err, "wrong password for wallet entered")
	} else if err != nil {
		return errors.Wrap(err, "could not decrypt keystore")
	}

	store := &accountStore{}
	if err := json.Unmarshal(enc, store); err != nil {
		return err
	}
	if len(store.PublicKeys) != len(store.PrivateKeys) {
		return errors.New("unequal number of public keys and private keys")
	}
	if len(store.PublicKeys) == 0 {
		return nil
	}
	km.accountsStore = store
	log.Info("getting public keys from wallet: ", km.accountsStore.PublicKeys)

	lock.Lock()
	for _, pubKey := range keystoreFile.DisabledPublicKeys {
		pubKeyBytes, err := hex.DecodeString(pubKey)
		if err != nil {
			lock.Unlock()
			return err
		}
		km.disabledPublicKeys[bytesutil.ToBytes48(pubKeyBytes)] = true
	}
	lock.Unlock()
	err = km.initializeKeysCachesFromKeystore()
	if err != nil {
		return errors.Wrap(err, "failed to initialize keys caches")
	}
	return err
}

// Initialize public and secret key caches that are used to speed up the functions
// FetchValidatingPublicKeys and Sign
func (km *Keymanager) initializeKeysCachesFromKeystore() error {
	lock.Lock()
	defer lock.Unlock()
	count := len(km.accountsStore.PrivateKeys)
	orderedPublicKeys = make([][48]byte, count)
	secretKeysCache = make(map[[48]byte]bls.SecretKey, count)
	for i, publicKey := range km.accountsStore.PublicKeys {
		publicKey48 := bytesutil.ToBytes48(publicKey)
		orderedPublicKeys[i] = publicKey48
		secretKey, err := herumi.SecretKeyFromBytes(km.accountsStore.PrivateKeys[i])
		if err != nil {
			return errors.Wrap(err, "failed to initialize keys caches from account keystore")
		}
		secretKeysCache[publicKey48] = secretKey
	}
	return nil
}


// Sign signs a message using a validator key.
func (km *Keymanager) Sign(slotInfo *SlotInfo, pubKeyIndex uint64) (bls.Signature, error) {

	slotInfoHash := slotInfo.Hash()
	publicKey := km.accountsStore.PublicKeys[pubKeyIndex]
	if publicKey == nil {
		return nil, errors.New("nil public key in request")
	}
	lock.RLock()
	secretKey, ok := secretKeysCache[bytesutil.ToBytes48(publicKey)]
	lock.RUnlock()
	if !ok {
		return nil, errors.New("no signing key found in keys cache")
	}
	return secretKey.Sign(slotInfoHash.Bytes()), nil
}

// VerifyBlockSigningRoot verifies the signing root of a block given it's public key, signature and domain.
func (km *Keymanager) VerifySignature(slotInfo *SlotInfo, pubKeyIndex uint64, signature bls.Signature) error {
	slotInfoHash := slotInfo.Hash()
	pub := km.accountsStore.PublicKeys[pubKeyIndex]

	publicKey, err := herumi.PublicKeyFromBytes(pub)
	if err != nil {
		return errors.Wrap(err, "could not convert bytes to public key")
	}

	if !signature.Verify(publicKey, slotInfoHash[:]) {
		return ErrSigFailedToVerify
	}
	return nil
}