package main

import (
	accManager "github.com/atif-konasl/eth-research/wallet"
)

func main() {
	config := accManager.Config{
		WalletDir: "./wallet/prysm-wallet-v2",
		KeymanagerKind: accManager.Kind(0),
		WalletPassword: "Konasl@123",
	}
	log.Info("opening bls keystore wallet. directory: ", config.WalletDir)
	wallet, err := accManager.OpenWallet(nil, &config)
	if err != nil {
		log.Errorf("failed to open wallet: %v", err)
		return
	}

	log.Info("setting up key manager with wallet....")
	keyManager, err := accManager.NewKeymanager(wallet)
	if err != nil {
		log.Errorf("failed to initiate key manager: %v", err)
		return
	}

	slotInfo := accManager.NewSlotInfo(2, 64, 5454)
	log.Info("creating dummy slot info for signing: ", slotInfo)

	signerPubKeyIndex := uint64(1)
	signature, err := keyManager.Sign(slotInfo, signerPubKeyIndex)
	if err != nil {
		log.Errorf("failed to verify signature with publicKeyIndex: %d error: %v", signerPubKeyIndex, err)
		return
	}
	log.Info("successfully generate signature: %s with publicKeyIndex: %d", signature.HexString(), signerPubKeyIndex)

	verifierPubKeyIndex := uint64(0)
	err = keyManager.VerifySignature(slotInfo, verifierPubKeyIndex, signature)
	if err != nil {
		log.Errorf("failed to verify signature: %s with publicKeyIndex: %d error: %v", signature.HexString(), verifierPubKeyIndex, err)
		return
	}
	log.Infof("successfully verify the signature: %s with publicKeyIndex: %d", signature.HexString(), verifierPubKeyIndex)
}