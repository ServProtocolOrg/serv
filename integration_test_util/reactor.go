package integration_test_util

import (
	"github.com/servprotocolorg/serv/v12/constants"
	sdk "github.com/cosmos/cosmos-sdk/types"

	// Force-load the tracer engines to trigger registration due to Go-Ethereum v1.10.15 changes
	_ "github.com/ethereum/go-ethereum/eth/tracers/js"
	_ "github.com/ethereum/go-ethereum/eth/tracers/native"
)

func init() {
	//goland:noinspection SpellCheckingInspection
	const prefix = constants.Bech32Prefix

	config := sdk.GetConfig()

	config.SetBech32PrefixForAccount(
		prefix,
		prefix+sdk.PrefixPublic,
	)

	config.SetBech32PrefixForValidator(
		prefix+sdk.PrefixValidator+sdk.PrefixOperator,
		prefix+sdk.PrefixValidator+sdk.PrefixOperator+sdk.PrefixPublic,
	)

	config.SetBech32PrefixForConsensusNode(
		prefix+sdk.PrefixValidator+sdk.PrefixConsensus,
		prefix+sdk.PrefixValidator+sdk.PrefixConsensus+sdk.PrefixPublic,
	)

	sdk.DefaultBondDenom = constants.BaseDenom
}
