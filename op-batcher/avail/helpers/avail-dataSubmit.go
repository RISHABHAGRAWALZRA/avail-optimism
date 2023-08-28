package avail

import (
	"fmt"

	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/ethereum-optimism/optimism/op-batcher/avail/internal/config"
)

var localNonce uint32 = 0

func getAccountNonce(accountNonce uint32) uint32 {
	if accountNonce > localNonce {
		localNonce = accountNonce
		return accountNonce
	}
	localNonce++
	return localNonce
}

// submitData creates a transaction and makes a Avail data submission
func SubmitData(data []byte) error {

	var config config.Config
	err := config.GetConfig("./avail/config.json")
	if err != nil {
		panic(fmt.Sprintf("cannot get config:%v", err))
	}

	//Intitializing variables
	ApiURL := config.ApiURL
	Seed := config.Seed
	AppID := config.AppID

	api, err := gsrpc.NewSubstrateAPI(ApiURL)
	if err != nil {
		return fmt.Errorf("cannot create api:%w", err)
	}

	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return fmt.Errorf("cannot get metadata:%w", err)
	}

	appID := 0
	// if app id is greater than 0 then it must be created before submitting data
	if AppID != 0 {
		appID = AppID
	}

	c, err := types.NewCall(meta, "DataAvailability.submit_data", types.NewBytes([]byte(data)))
	if err != nil {
		return fmt.Errorf("cannot create new call:%w", err)
	}

	// Create the extrinsic
	ext := types.NewExtrinsic(c)

	genesisHash, err := api.RPC.Chain.GetBlockHash(0)
	if err != nil {
		return fmt.Errorf("cannot get block hash:%w", err)
	}

	rv, err := api.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		return fmt.Errorf("cannot get runtime version:%w", err)
	}

	keyringPair, err := signature.KeyringPairFromSecret(Seed, 42)
	if err != nil {
		return fmt.Errorf("cannot create LeyPair:%w", err)
	}
	//fmt.Println(keyringPair.Address)

	key, err := types.CreateStorageKey(meta, "System", "Account", keyringPair.PublicKey)
	if err != nil {
		return fmt.Errorf("cannot create storage key:%w", err)
	}

	var accountInfo types.AccountInfo
	ok, err := api.RPC.State.GetStorageLatest(key, &accountInfo)
	if err != nil || !ok {
		return fmt.Errorf("cannot get latest storage:%w", err)
	}

	nonce := getAccountNonce(uint32(accountInfo.Nonce))
	//fmt.Println("Nonce from localDatabase:", nonce, "    ::::::::   from acountInfo:", accountInfo.Nonce)
	o := types.SignatureOptions{
		BlockHash:          genesisHash,
		Era:                types.ExtrinsicEra{IsMortalEra: false},
		GenesisHash:        genesisHash,
		Nonce:              types.NewUCompactFromUInt(uint64(nonce)),
		SpecVersion:        rv.SpecVersion,
		Tip:                types.NewUCompactFromUInt(0),
		AppID:              types.NewUCompactFromUInt(uint64(appID)),
		TransactionVersion: rv.TransactionVersion,
	}

	// Sign the transaction using Alice's default account
	err = ext.Sign(keyringPair, o)
	if err != nil {
		return fmt.Errorf("cannot sign:%w", err)
	}

	// Send the extrinsic
	hash, err := api.RPC.Author.SubmitExtrinsic(ext)
	if err != nil {
		return fmt.Errorf("cannot submit extrinsic:%w", err)
	}

	fmt.Printf("Data of length %v submitted by op-batcher using appID %v sent with hash %#x\n", len(data), appID, hash)

	return nil
}
