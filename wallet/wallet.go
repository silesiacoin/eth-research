package wallet

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/atif-konasl/eth-research/fileutil"
	"github.com/pkg/errors"
)

// Kind defines an enum for either imported, derived, or remote-signing
// keystores for Prysm wallets.
type Kind int

const (
	// Imported keymanager defines an on-disk, encrypted keystore-capable store.
	Imported Kind = iota
	// Derived keymanager using a hierarchical-deterministic algorithm.
	Derived
	// Remote keymanager capable of remote-signing data.
	Remote
)

const (
	// KeymanagerConfigFileName for the keymanager used by the wallet: imported, derived, or remote.
	KeymanagerConfigFileName = "keymanageropts.json"
	// NewWalletPasswordPromptText for wallet creation.
	NewWalletPasswordPromptText = "New wallet password"
	// WalletPasswordPromptText for wallet unlocking.
	PasswordPromptText = "Wallet password"
	// ConfirmPasswordPromptText for confirming a wallet password.
	ConfirmPasswordPromptText = "Confirm password"
	// DefaultWalletPasswordFile used to store a wallet password with appropriate permissions
	// if a user signs up via the Prysm web UI via RPC.
	DefaultWalletPasswordFile = "walletpassword.txt"
	// CheckExistsErrMsg for when there is an error while checking for a wallet
	CheckExistsErrMsg = "could not check if wallet exists"
	// CheckValidityErrMsg for when there is an error while checking wallet validity
	CheckValidityErrMsg = "could not check if wallet is valid"
	// InvalidWalletErrMsg for when a directory does not contain a valid wallet
	InvalidWalletErrMsg = "directory does not contain valid wallet"
)

// Config to open a wallet programmatically.
type Config struct {
	WalletDir      string
	KeymanagerKind Kind
	WalletPassword string
}

// Wallet is a primitive in Prysm's account management which
// has the capability of creating new accounts, reading existing accounts,
// and providing secure access to eth2 secrets depending on an
// associated keymanager (either imported, derived, or remote signing enabled).
type Wallet struct {
	walletDir      string
	accountsPath   string
	configFilePath string
	walletPassword string
	keymanagerKind Kind
}

// New creates a struct from config values.
func NewWallet(cfg *Config) *Wallet {
	accountsPath := filepath.Join(cfg.WalletDir, cfg.KeymanagerKind.String())
	return &Wallet{
		walletDir:      cfg.WalletDir,
		accountsPath:   accountsPath,
		keymanagerKind: cfg.KeymanagerKind,
		walletPassword: cfg.WalletPassword,
	}
}

// ReadFileAtPath within the wallet directory given the desired path and filename.
func (w *Wallet) ReadFileAtPath(filePath, fileName string) ([]byte, error) {
	accountPath := filepath.Join(w.accountsPath, filePath)
	hasDir, err := fileutil.HasDir(accountPath)
	if err != nil {
		return nil, err
	}
	if !hasDir {
		if err := fileutil.MkdirAll(accountPath); err != nil {
			return nil, errors.Wrapf(err, "could not create path: %s", accountPath)
		}
	}
	fullPath := filepath.Join(accountPath, fileName)
	matches, err := filepath.Glob(fullPath)
	if err != nil {
		return []byte{}, errors.Wrap(err, "could not find file")
	}
	if len(matches) == 0 {
		return []byte{}, fmt.Errorf("no files found in path: %s", fullPath)
	}
	rawData, err := ioutil.ReadFile(matches[0])
	if err != nil {
		return nil, errors.Wrapf(err, "could not read path: %s", filePath)
	}
	return rawData, nil
}

// OpenWallet instantiates a wallet from a specified path. It checks the
// type of keymanager associated with the wallet by reading files in the wallet
// path, if applicable. If a wallet does not exist, returns an appropriate error.
func OpenWallet(_ context.Context, cfg *Config) (*Wallet, error) {
	exists, err := Exists(cfg.WalletDir)
	if err != nil {
		return nil, errors.Wrap(err, CheckExistsErrMsg)
	}
	if !exists {
		return nil, ErrNoWalletFound
	}
	valid, err := IsValid(cfg.WalletDir)
	// ErrNoWalletFound represents both a directory that does not exist as well as an empty directory
	if errors.Is(err, ErrNoWalletFound) {
		return nil, ErrNoWalletFound
	}
	if err != nil {
		return nil, errors.Wrap(err, CheckValidityErrMsg)
	}
	if !valid {
		return nil, errors.New(InvalidWalletErrMsg)
	}

	keymanagerKind, err := readKeymanagerKindFromWalletPath(cfg.WalletDir)
	if err != nil {
		return nil, errors.Wrap(err, "could not read keymanager kind for wallet")
	}
	accountsPath := filepath.Join(cfg.WalletDir, keymanagerKind.String())
	return &Wallet{
		walletDir:      cfg.WalletDir,
		accountsPath:   accountsPath,
		keymanagerKind: keymanagerKind,
		walletPassword: cfg.WalletPassword,
	}, nil
}



func readKeymanagerKindFromWalletPath(walletPath string) (Kind, error) {
	walletItem, err := os.Open(walletPath)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := walletItem.Close(); err != nil {
			log.WithField(
				"path", walletPath,
			).Errorf("Could not close wallet directory: %v", err)
		}
	}()
	list, err := walletItem.Readdirnames(0) // 0 to read all files and folders.
	if err != nil {
		return 0, fmt.Errorf("could not read files in directory: %s", walletPath)
	}
	for _, n := range list {
		keymanagerKind, err := ParseKind(n)
		if err == nil {
			return keymanagerKind, nil
		}
	}
	return 0, errors.New("no keymanager folder (imported, remote, derived) found in wallet path")
}

// Exists checks if directory at walletDir exists
func Exists(walletDir string) (bool, error) {
	dirExists, err := fileutil.HasDir(walletDir)
	if err != nil {
		return false, errors.Wrap(err, "could not parse wallet directory")
	}
	isValid, err := IsValid(walletDir)
	if errors.Is(err, ErrNoWalletFound) {
		return false, nil
	} else if err != nil {
		return false, errors.Wrap(err, "could not check if dir is valid")
	}
	return dirExists && isValid, nil
}

// IsValid checks if a folder contains a single key directory such as `derived`, `remote` or `imported`.
// Returns true if one of those subdirectories exist, false otherwise.
func IsValid(walletDir string) (bool, error) {
	expanded, err := fileutil.ExpandPath(walletDir)
	if err != nil {
		return false, err
	}
	f, err := os.Open(expanded)
	if err != nil {
		if strings.Contains(err.Error(), "no such file") ||
			strings.Contains(err.Error(), "cannot find the file") ||
			strings.Contains(err.Error(), "cannot find the path") {
			return false, nil
		}
		return false, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Debugf("Could not close directory: %s", expanded)
		}
	}()
	names, err := f.Readdirnames(-1)
	if err != nil {
		return false, err
	}

	if len(names) == 0 {
		return false, ErrNoWalletFound
	}

	// Count how many wallet types we have in the directory
	numWalletTypes := 0
	for _, name := range names {
		// Nil error means input name is `derived`, `remote` or `imported`
		_, err = ParseKind(name)
		if err == nil {
			numWalletTypes++
		}
	}
	return numWalletTypes == 1, nil
}


// ParseKind from a raw string, returning a keymanager kind.
func ParseKind(k string) (Kind, error) {
	switch k {
	case "derived":
		return Derived, nil
	case "direct":
		return Imported, nil
	case "remote":
		return Remote, nil
	default:
		return 0, fmt.Errorf("%s is not an allowed keymanager", k)
	}
}

// String marshals a keymanager kind to a string value.
func (k Kind) String() string {
	switch k {
	case Derived:
		return "derived"
	case Imported:
		return "direct"
	case Remote:
		return "remote"
	default:
		return fmt.Sprintf("%d", int(k))
	}
}