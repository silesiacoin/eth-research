package wallet


import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/atif-konasl/eth-research/testutil/require"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(ioutil.Discard)
}

func Test_Exists_RandomFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallet")

	exists, err := Exists(path)
	require.Equal(t, false, exists)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(path+"/direct", 0700), "Failed to create directory")

	exists, err = Exists(path)
	require.NoError(t, err)
	require.Equal(t, true, exists)
}

func Test_IsValid_RandomFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallet")
	valid, err := IsValid(path)
	require.NoError(t, err)
	require.Equal(t, false, valid)

	require.NoError(t, os.MkdirAll(path, 0700), "Failed to create directory")

	valid, err = IsValid(path)
	require.ErrorContains(t, "no wallet found", err)
	require.Equal(t, false, valid)

	walletDir := filepath.Join(path, "direct")
	require.NoError(t, os.MkdirAll(walletDir, 0700), "Failed to create directory")

	valid, err = IsValid(path)
	require.NoError(t, err)
	require.Equal(t, true, valid)
}

func Test_OpenWallet(t *testing.T) {
	config := Config{
		WalletDir: "./prysm-wallet-v2",
		KeymanagerKind: Kind(0),
		WalletPassword: "Konasl@123",
	}

	wallet, err := OpenWallet(nil, &config)
	require.NoError(t, err)
	require.Equal(t, "./prysm-wallet-v2", wallet.walletDir)
	require.Equal(t, "prysm-wallet-v2/direct", wallet.accountsPath)
	require.Equal(t, Kind(0), wallet.keymanagerKind)
	require.Equal(t, "Lukso1105072/", wallet.walletPassword)
}
