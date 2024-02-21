package backend

import (
	"fmt"
	"math/big"

	"github.com/servprotocolorg/serv/v12/indexer"
	"github.com/servprotocolorg/serv/v12/rpc/backend/mocks"
	rpctypes "github.com/servprotocolorg/serv/v12/rpc/types"
	evertypes "github.com/servprotocolorg/serv/v12/types"
	evmtypes "github.com/servprotocolorg/serv/v12/x/evm/types"
	dbm "github.com/cometbft/cometbft-db"
	abci "github.com/cometbft/cometbft/abci/types"
	tmlog "github.com/cometbft/cometbft/libs/log"
	tmrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func (suite *BackendTestSuite) TestGetTransactionByHash() {
	msgEthereumTx, _ := suite.buildEthereumTx()
	txHash := msgEthereumTx.AsTransaction().Hash()

	txBz := suite.signAndEncodeEthTx(msgEthereumTx)
	block := &types.Block{Header: types.Header{Height: 1, ChainID: "test"}, Data: types.Data{Txs: []types.Tx{txBz}}}
	responseDeliver := []*abci.ResponseDeliverTx{
		{
			Code: 0,
			Events: []abci.Event{
				{Type: evmtypes.EventTypeEthereumTx, Attributes: []abci.EventAttribute{
					{Key: "ethereumTxHash", Value: txHash.Hex()},
					{Key: "txIndex", Value: "0"},
					{Key: "amount", Value: "1000"},
					{Key: "txGasUsed", Value: "21000"},
					{Key: "txHash", Value: ""},
					{Key: "recipient", Value: ""},
				}},
			},
		},
	}

	rpcTransaction, _ := rpctypes.NewRPCTransaction(msgEthereumTx.AsTransaction(), common.Hash{}, 0, 0, big.NewInt(1), suite.backend.chainID)

	testCases := []struct {
		name         string
		registerMock func()
		tx           *evmtypes.MsgEthereumTx
		expRPCTx     *rpctypes.RPCTransaction
		expPass      bool
	}{
		{
			"fail - Block error",
			func() {
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				RegisterBlockError(client, 1)
			},
			msgEthereumTx,
			rpcTransaction,
			false,
		},
		{
			"fail - Block Result error",
			func() {
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				_, err := RegisterBlock(client, 1, txBz)
				suite.Require().NoError(err)
				RegisterBlockResultsError(client, 1)
			},
			msgEthereumTx,
			nil,
			true,
		},
		{
			"pass - Base fee error",
			func() {
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				queryClient := suite.backend.queryClient.QueryClient.(*mocks.EVMQueryClient)
				_, err := RegisterBlock(client, 1, txBz)
				suite.Require().NoError(err)
				_, err = RegisterBlockResults(client, 1)
				suite.Require().NoError(err)
				RegisterBaseFeeError(queryClient)
			},
			msgEthereumTx,
			rpcTransaction,
			true,
		},
		{
			"pass - Transaction found and returned",
			func() {
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				queryClient := suite.backend.queryClient.QueryClient.(*mocks.EVMQueryClient)
				_, err := RegisterBlock(client, 1, txBz)
				suite.Require().NoError(err)
				_, err = RegisterBlockResults(client, 1)
				suite.Require().NoError(err)
				RegisterBaseFee(queryClient, sdk.NewInt(1))
			},
			msgEthereumTx,
			rpcTransaction,
			true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			tc.registerMock()

			db := dbm.NewMemDB()
			suite.backend.indexer = indexer.NewKVIndexer(db, tmlog.NewNopLogger(), suite.backend.clientCtx)
			err := suite.backend.indexer.IndexBlock(block, responseDeliver)
			suite.Require().NoError(err)
			suite.backend.indexer.Ready()

			rpcTx, err := suite.backend.GetTransactionByHash(common.HexToHash(tc.tx.Hash))

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(rpcTx, tc.expRPCTx)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *BackendTestSuite) TestGetTransactionsByHashPending() {
	msgEthereumTx, bz := suite.buildEthereumTx()
	rpcTransaction, _ := rpctypes.NewRPCTransaction(msgEthereumTx.AsTransaction(), common.Hash{}, 0, 0, big.NewInt(1), suite.backend.chainID)

	testCases := []struct {
		name         string
		registerMock func()
		tx           *evmtypes.MsgEthereumTx
		expRPCTx     *rpctypes.RPCTransaction
		expPass      bool
	}{
		{
			"fail - Pending transactions returns error",
			func() {
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				RegisterUnconfirmedTxsError(client, nil)
			},
			msgEthereumTx,
			nil,
			true,
		},
		{
			"fail - Tx not found return nil",
			func() {
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				RegisterUnconfirmedTxs(client, nil, nil)
			},
			msgEthereumTx,
			nil,
			true,
		},
		{
			"pass - Tx found and returned",
			func() {
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				RegisterUnconfirmedTxs(client, nil, types.Txs{bz})
			},
			msgEthereumTx,
			rpcTransaction,
			true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			tc.registerMock()

			rpcTx, err := suite.backend.getTransactionByHashPending(common.HexToHash(tc.tx.Hash))

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(rpcTx, tc.expRPCTx)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *BackendTestSuite) TestGetTxByEthHash() {
	msgEthereumTx, _ := suite.buildEthereumTx()
	rpcTransaction, _ := rpctypes.NewRPCTransaction(msgEthereumTx.AsTransaction(), common.Hash{}, 0, 0, big.NewInt(1), suite.backend.chainID)

	testCases := []struct {
		name         string
		registerMock func()
		tx           *evmtypes.MsgEthereumTx
		expRPCTx     *rpctypes.RPCTransaction
		expPass      bool
	}{
		{
			"fail - Indexer disabled can't find transaction",
			func() {
				indexer := suite.backend.indexer.(*mocks.EVMTxIndexer)
				RegisterIndexerGetByTxHashErr(indexer, msgEthereumTx.AsTransaction().Hash())
			},
			msgEthereumTx,
			rpcTransaction,
			false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			tc.registerMock()

			rpcTx, err := suite.backend.GetTxByEthHash(common.HexToHash(tc.tx.Hash))

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(rpcTx, tc.expRPCTx)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *BackendTestSuite) TestGetTransactionByBlockHashAndIndex() {
	_, bz := suite.buildEthereumTx()

	testCases := []struct {
		name         string
		registerMock func()
		blockHash    common.Hash
		expRPCTx     *rpctypes.RPCTransaction
		expPass      bool
	}{
		{
			"pass - block not found",
			func() {
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				RegisterBlockByHashError(client, common.Hash{}, bz)
			},
			common.Hash{},
			nil,
			true,
		},
		{
			"pass - Block results error",
			func() {
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				_, err := RegisterBlockByHash(client, common.Hash{}, bz)
				suite.Require().NoError(err)
				RegisterBlockResultsError(client, 1)
			},
			common.Hash{},
			nil,
			true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			tc.registerMock()

			rpcTx, err := suite.backend.GetTransactionByBlockHashAndIndex(tc.blockHash, 1)

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(rpcTx, tc.expRPCTx)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *BackendTestSuite) TestGetTransactionByBlockAndIndex() {
	msgEthTx, _ := suite.buildEthereumTx()
	msgEthTx, bz := suite.signMsgEthTx(msgEthTx)

	defaultBlock := types.MakeBlock(1, []types.Tx{bz}, nil, nil)
	defaultResponseDeliverTx := []*abci.ResponseDeliverTx{
		{
			Code: 0,
			Events: []abci.Event{
				{Type: evmtypes.EventTypeEthereumTx, Attributes: []abci.EventAttribute{
					{Key: "ethereumTxHash", Value: common.HexToHash(msgEthTx.Hash).Hex()},
					{Key: "txIndex", Value: "0"},
					{Key: "amount", Value: "1000"},
					{Key: "txGasUsed", Value: "21000"},
					{Key: "txHash", Value: ""},
					{Key: "recipient", Value: ""},
				}},
			},
		},
	}

	txFromMsg, _ := rpctypes.NewTransactionFromMsg(
		msgEthTx,
		common.BytesToHash(defaultBlock.Hash().Bytes()),
		1,
		0,
		big.NewInt(1),
		suite.backend.chainID,
	)
	testCases := []struct {
		name         string
		registerMock func()
		block        *tmrpctypes.ResultBlock
		idx          hexutil.Uint
		expRPCTx     *rpctypes.RPCTransaction
		expPass      bool
	}{
		{
			"pass - block txs index out of bound",
			func() {
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				_, err := RegisterBlockResults(client, 1)
				suite.Require().NoError(err)

				indexer := suite.backend.indexer.(*mocks.EVMTxIndexer)
				RegisterIndexerGetByBlockAndIndexError(indexer, 1, 1)
			},
			&tmrpctypes.ResultBlock{Block: types.MakeBlock(1, []types.Tx{bz}, nil, nil)},
			1,
			nil,
			true,
		},
		{
			"pass - Can't fetch base fee",
			func() {
				queryClient := suite.backend.queryClient.QueryClient.(*mocks.EVMQueryClient)
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				_, err := RegisterBlockResults(client, 1)
				suite.Require().NoError(err)
				RegisterBaseFeeError(queryClient)

				indexer := suite.backend.indexer.(*mocks.EVMTxIndexer)
				RegisterIndexerGetByBlockAndIndex(indexer, 1, 0)
			},
			&tmrpctypes.ResultBlock{Block: defaultBlock},
			0,
			txFromMsg,
			true,
		},
		{
			"pass - Gets Tx by transaction index",
			func() {
				queryClient := suite.backend.queryClient.QueryClient.(*mocks.EVMQueryClient)
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				db := dbm.NewMemDB()
				suite.backend.indexer = indexer.NewKVIndexer(db, tmlog.NewNopLogger(), suite.backend.clientCtx)
				txBz := suite.signAndEncodeEthTx(msgEthTx)
				block := &types.Block{Header: types.Header{Height: 1, ChainID: "test"}, Data: types.Data{Txs: []types.Tx{txBz}}}
				err := suite.backend.indexer.IndexBlock(block, defaultResponseDeliverTx)
				suite.Require().NoError(err)
				suite.backend.indexer.Ready()
				_, err = RegisterBlockResults(client, 1)
				suite.Require().NoError(err)
				RegisterBaseFee(queryClient, sdk.NewInt(1))
			},
			&tmrpctypes.ResultBlock{Block: defaultBlock},
			0,
			txFromMsg,
			true,
		},
		{
			"pass - returns the Ethereum format transaction by the Ethereum hash",
			func() {
				queryClient := suite.backend.queryClient.QueryClient.(*mocks.EVMQueryClient)
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				_, err := RegisterBlockResults(client, 1)
				suite.Require().NoError(err)
				RegisterBaseFee(queryClient, sdk.NewInt(1))

				indexer := suite.backend.indexer.(*mocks.EVMTxIndexer)
				RegisterIndexerGetByBlockAndIndex(indexer, 1, 0)
			},
			&tmrpctypes.ResultBlock{Block: defaultBlock},
			0,
			txFromMsg,
			true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			tc.registerMock()

			rpcTx, err := suite.backend.GetTransactionByBlockAndIndex(tc.block, tc.idx)

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(rpcTx, tc.expRPCTx)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *BackendTestSuite) TestGetTransactionByBlockNumberAndIndex() {
	msgEthTx, bz := suite.buildEthereumTx()
	defaultBlock := types.MakeBlock(1, []types.Tx{bz}, nil, nil)
	txFromMsg, _ := rpctypes.NewTransactionFromMsg(
		msgEthTx,
		common.BytesToHash(defaultBlock.Hash().Bytes()),
		1,
		0,
		big.NewInt(1),
		suite.backend.chainID,
	)
	testCases := []struct {
		name         string
		registerMock func()
		blockNum     rpctypes.BlockNumber
		idx          hexutil.Uint
		expRPCTx     *rpctypes.RPCTransaction
		expPass      bool
	}{
		{
			"fail -  block not found return nil",
			func() {
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				RegisterBlockError(client, 1)
			},
			0,
			0,
			nil,
			true,
		},
		{
			"pass - returns the transaction identified by block number and index",
			func() {
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				queryClient := suite.backend.queryClient.QueryClient.(*mocks.EVMQueryClient)
				_, err := RegisterBlock(client, 1, bz)
				suite.Require().NoError(err)
				_, err = RegisterBlockResults(client, 1)
				suite.Require().NoError(err)
				RegisterBaseFee(queryClient, sdk.NewInt(1))

				indexer := suite.backend.indexer.(*mocks.EVMTxIndexer)
				RegisterIndexerGetByBlockAndIndex(indexer, 1, 0)
			},
			0,
			0,
			txFromMsg,
			true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			tc.registerMock()

			rpcTx, err := suite.backend.GetTransactionByBlockNumberAndIndex(tc.blockNum, tc.idx)
			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(rpcTx, tc.expRPCTx)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *BackendTestSuite) TestGetTransactionByTxIndex() {
	testCases := []struct {
		name         string
		registerMock func()
		height       int64
		index        uint
		expTxResult  *evertypes.TxResult
		expPass      bool
	}{
		{
			"fail - Ethereum tx with query not found",
			func() {
				indexer := suite.backend.indexer.(*mocks.EVMTxIndexer)
				RegisterIndexerGetByBlockAndIndexError(indexer, 1, 0)
			},
			1,
			0,
			&evertypes.TxResult{},
			false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			tc.registerMock()

			txResults, err := suite.backend.GetTxByTxIndex(tc.height, tc.index)

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(txResults, tc.expTxResult)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *BackendTestSuite) TestQueryTendermintTxIndexer() {
	testCases := []struct {
		name         string
		registerMock func()
		txGetter     func(*rpctypes.ParsedTxs) *rpctypes.ParsedTx
		query        string
		expTxResult  *evertypes.TxResult
		expPass      bool
	}{
		{
			"fail - Ethereum tx with query not found",
			func() {
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				RegisterTxSearchEmpty(client, "")
			},
			func(txs *rpctypes.ParsedTxs) *rpctypes.ParsedTx {
				return &rpctypes.ParsedTx{}
			},
			"",
			&evertypes.TxResult{},
			false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			tc.registerMock()

			txResults, err := suite.backend.queryTendermintTxIndexer(tc.query, tc.txGetter)

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(txResults, tc.expTxResult)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *BackendTestSuite) TestGetTransactionReceipt() {
	msgEthereumTx, _ := suite.buildEthereumTx()
	txHash := msgEthereumTx.AsTransaction().Hash()

	txBz := suite.signAndEncodeEthTx(msgEthereumTx)

	testCases := []struct {
		name         string
		registerMock func()
		tx           *evmtypes.MsgEthereumTx
		block        *types.Block
		blockResult  []*abci.ResponseDeliverTx
		expTxReceipt *rpctypes.RPCReceipt
		expPass      bool
	}{
		{
			"fail - Receipts do not match",
			func() {
				queryClient := suite.backend.queryClient.QueryClient.(*mocks.EVMQueryClient)
				client := suite.backend.clientCtx.Client.(*mocks.Client)
				RegisterParamsWithoutHeader(queryClient, 1)
				_, err := RegisterBlock(client, 1, txBz)
				suite.Require().NoError(err)
				_, err = RegisterBlockResults(client, 1)
				suite.Require().NoError(err)
			},
			msgEthereumTx,
			&types.Block{Header: types.Header{Height: 1}, Data: types.Data{Txs: []types.Tx{txBz}}},
			[]*abci.ResponseDeliverTx{
				{
					Code: 0,
					Events: []abci.Event{
						{Type: evmtypes.EventTypeEthereumTx, Attributes: []abci.EventAttribute{
							{Key: "ethereumTxHash", Value: txHash.Hex()},
							{Key: "txIndex", Value: "0"},
							{Key: "amount", Value: "1000"},
							{Key: "txGasUsed", Value: "21000"},
							{Key: "txHash", Value: ""},
							{Key: "recipient", Value: "0x775b87ef5D82ca211811C1a02CE0fE0CA3a455d7"},
						}},
					},
				},
			},
			(*rpctypes.RPCReceipt)(nil),
			false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			tc.registerMock()

			db := dbm.NewMemDB()
			suite.backend.indexer = indexer.NewKVIndexer(db, tmlog.NewNopLogger(), suite.backend.clientCtx)
			err := suite.backend.indexer.IndexBlock(tc.block, tc.blockResult)
			suite.Require().NoError(err)
			suite.backend.indexer.Ready()

			txReceipt, err := suite.backend.GetTransactionReceipt(common.HexToHash(tc.tx.Hash))
			if tc.expPass {
				suite.Require().NoError(err)
				suite.Equal(tc.expTxReceipt, txReceipt)
			} else {
				suite.NotEqual(tc.expTxReceipt, txReceipt)
			}
		})
	}

	testCasesIndexerErr := []struct {
		name         string
		registerMock func(txHash common.Hash)
		tx           *evmtypes.MsgEthereumTx
		expErr       bool
	}{
		{
			name: "fail - indexer returns error",
			registerMock: func(txHash common.Hash) {
				indexer := suite.backend.indexer.(*mocks.EVMTxIndexer)
				RegisterIndexerGetByTxHashErr(indexer, txHash)
			},
			tx:     msgEthereumTx,
			expErr: false,
		},
	}
	for _, tc := range testCasesIndexerErr {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			signedTxHash := common.HexToHash(tc.tx.Hash)
			tc.registerMock(signedTxHash)

			receipt, err := suite.backend.GetTransactionReceipt(signedTxHash)
			if tc.expErr {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}
			suite.Nil(receipt)
		})
	}
}

func (suite *BackendTestSuite) TestGetGasUsed() {
	origin := suite.backend.cfg.JSONRPC.FixRevertGasRefundHeight
	testCases := []struct {
		name                     string
		fixRevertGasRefundHeight int64
		txResult                 *evertypes.TxResult
		price                    *big.Int
		gas                      uint64
		exp                      uint64
	}{
		{
			"success txResult",
			1,
			&evertypes.TxResult{
				Height:  1,
				Failed:  false,
				GasUsed: 53026,
			},
			new(big.Int).SetUint64(0),
			0,
			53026,
		},
		{
			"fail txResult before cap",
			2,
			&evertypes.TxResult{
				Height:  1,
				Failed:  true,
				GasUsed: 53026,
			},
			new(big.Int).SetUint64(200000),
			5000000000000,
			1000000000000000000,
		},
		{
			"fail txResult after cap",
			2,
			&evertypes.TxResult{
				Height:  3,
				Failed:  true,
				GasUsed: 53026,
			},
			new(big.Int).SetUint64(200000),
			5000000000000,
			53026,
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.backend.cfg.JSONRPC.FixRevertGasRefundHeight = tc.fixRevertGasRefundHeight
			suite.Require().Equal(tc.exp, suite.backend.GetGasUsed(tc.txResult, tc.price, tc.gas))
			suite.backend.cfg.JSONRPC.FixRevertGasRefundHeight = origin
		})
	}
}
