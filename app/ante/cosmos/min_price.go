package cosmos

import (
	"math/big"

	errorsmod "cosmossdk.io/errors"
	evmante "github.com/servprotocolorg/serv/v12/app/ante/evm"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

// MinGasPriceDecorator will check if the transaction's fee is at least as large
// as the MinGasPrices param. If fee is too low, decorator returns error and tx
// is rejected. This applies for both CheckTx and DeliverTx
// If fee is high enough, then call next AnteHandler
// CONTRACT: Tx must implement FeeTx to use MinGasPriceDecorator
type MinGasPriceDecorator struct {
	feesKeeper evmante.FeeMarketKeeper
	evmKeeper  evmante.EVMKeeper
}

// NewMinGasPriceDecorator creates a new MinGasPriceDecorator instance used only for
// Cosmos transactions.
func NewMinGasPriceDecorator(fk evmante.FeeMarketKeeper, ek evmante.EVMKeeper) MinGasPriceDecorator {
	return MinGasPriceDecorator{feesKeeper: fk, evmKeeper: ek}
}

func (mpd MinGasPriceDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return ctx, errorsmod.Wrapf(errortypes.ErrInvalidType, "invalid transaction type %T, expected sdk.FeeTx", tx)
	}

	feeCoins := feeTx.GetFee()
	evmParams := mpd.evmKeeper.GetParams(ctx)
	evmDenom := evmParams.GetEvmDenom()

	if !isNoFeeProvidedOrOnlyEvmDenom(feeCoins, evmDenom) {
		return ctx, errorsmod.Wrapf(errortypes.ErrInvalidCoins, "fee can only be %s", evmDenom)
	}

	minGasPrice := mpd.feesKeeper.GetParams(ctx).MinGasPrice

	// Short-circuit if min gas price is 0 or if simulating
	if minGasPrice.IsZero() || simulate {
		return next(ctx, tx, simulate)
	}

	minGasPrices := sdk.DecCoins{
		{
			Denom:  evmDenom,
			Amount: minGasPrice,
		},
	}

	gas := feeTx.GetGas()

	requiredFees := make(sdk.Coins, 0)

	// Determine the required fees by multiplying each required minimum gas
	// price by the gas limit, where fee = ceil(minGasPrice * gasLimit).
	gasLimit := sdk.NewDecFromBigInt(new(big.Int).SetUint64(gas))

	for _, gp := range minGasPrices {
		fee := gp.Amount.Mul(gasLimit).Ceil().RoundInt()
		if fee.IsPositive() {
			requiredFees = requiredFees.Add(sdk.Coin{Denom: gp.Denom, Amount: fee})
		}
	}

	// Fees not provided (or flag "auto"). Then use the base fee to make the check pass
	if feeCoins == nil {
		return ctx, errorsmod.Wrapf(errortypes.ErrInsufficientFee,
			"fee not provided. Please use the --fees flag or the --gas-price flag along with the --gas flag to estimate the fee. The minimun global fee for this tx is: %s",
			requiredFees)
	}

	if !feeCoins.IsAnyGTE(requiredFees) {
		return ctx, errorsmod.Wrapf(errortypes.ErrInsufficientFee,
			"provided fee < minimum global fee (%s < %s). Please increase the gas price.",
			feeCoins,
			requiredFees)
	}

	return next(ctx, tx, simulate)
}

// isNoFeeProvidedOrOnlyEvmDenom returns true if fees is empty or only accept one fee denom which is the evm denom
func isNoFeeProvidedOrOnlyEvmDenom(fees sdk.Coins, evmDenom string) bool {
	if len(fees) == 0 {
		return true
	}

	if len(fees) > 1 {
		return false
	}

	return fees[0].Denom == evmDenom
}
