package wallet

import (
	"github.com/atif-konasl/eth-research/testutil/require"
	"testing"
)

func Test_InitializeAccountKeystore(t *testing.T) {
	config := Config{
		WalletDir: "./prysm-wallet-v2",
		KeymanagerKind: Kind(0),
		WalletPassword: "Konasl@123",
	}

	wallet, err := OpenWallet(nil, &config)
	require.NoError(t, err)

	keyManager, err := NewKeymanager(wallet)
	require.NoError(t, err)
	require.Equal(t, 2, len(keyManager.accountsStore.PublicKeys))
}
