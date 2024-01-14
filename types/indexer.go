//go:generate mockery --name EVMTxIndexer

package types

import (
	abci "github.com/cometbft/cometbft/abci/types"
	tmtypes "github.com/cometbft/cometbft/types"
	"github.com/ethereum/go-ethereum/common"
)

// EVMTxIndexer defines the interface of the custom ETH-Tx indexer.
type EVMTxIndexer interface {
	// LastIndexedBlock returns the last block number which was indexed and flushed into database.
	// Returns -1 if db is empty.
	LastIndexedBlock() (int64, error)

	// IndexBlock indexes all ETH Txs of the block.
	// Notes: no guarantee data is flushed into database after this function returns, it might be flushed at later point.
	IndexBlock(*tmtypes.Block, []*abci.ResponseDeliverTx) error

	// GetByTxHash returns nil if tx not found.
	GetByTxHash(common.Hash) (*TxResult, error)

	// GetByBlockAndIndex returns nil if tx not found.
	GetByBlockAndIndex(int64, int32) (*TxResult, error)
}
