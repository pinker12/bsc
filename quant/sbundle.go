package quant

import (
	"errors"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// SBundle is a bundle of transactions that must be executed atomically
// unlike ordinary bundle it also supports refunds
type SBundle struct {
	Inclusion BundleInclusion
	Body      []BundleBody
	Validity  BundleValidity

	hash atomic.Value
}

type BundleInclusion struct {
	BlockNumber    uint64
	MaxBlockNumber uint64
}

type BundleBody struct {
	Tx        *types.Transaction
	Bundle    *SBundle
	CanRevert bool
}

type BundleValidity struct {
	Refund       []RefundConstraint `json:"refund,omitempty"`
	RefundConfig []RefundConfig     `json:"refundConfig,omitempty"`
}

type RefundConstraint struct {
	BodyIdx int `json:"bodyIdx"`
	Percent int `json:"percent"`
}

type RefundConfig struct {
	Address common.Address `json:"address"`
	Percent int            `json:"percent"`
}

type SimBundleResult struct {
	TotalProfit     *uint256.Int
	RefundableValue *uint256.Int
	GasUsed         uint64
	MevGasPrice     *uint256.Int
	BodyLogs        []SimBundleBodyLogs
	Revert          []byte
	ExecError       string
}

type SimBundleBodyLogs struct {
	TxLogs     []*types.Log        `json:"txLogs,omitempty"`
	BundleLogs []SimBundleBodyLogs `json:"bundleLogs,omitempty"`
}

func NewSimBundleResult() SimBundleResult {
	return SimBundleResult{
		TotalProfit:     uint256.NewInt(0),
		RefundableValue: uint256.NewInt(0),
		GasUsed:         0,
		MevGasPrice:     uint256.NewInt(0),
		BodyLogs:        nil,
	}
}

func MakeBundle(txs []*types.Transaction, bloclNumber uint64, MaxBloclNumber uint64) *SBundle {
	bundle := &SBundle{
		Inclusion: BundleInclusion{
			BlockNumber:    bloclNumber,
			MaxBlockNumber: MaxBloclNumber,
		},
	}
	for _, tx := range txs {
		bundle.Body = append(bundle.Body, BundleBody{Tx: tx})
	}

	return bundle
}

// SimBundle simulates a bundle and returns the result
// Arguments are the same as in ApplyTransaction with the same change semantics:
// - statedb is modified
// - header is not modified
// - gp is modified
// - usedGas is modified (by txs that were applied)
// Payout transactions will not be applied to the state.
// GasUsed in return will include the gas that might be used by the payout txs.
func SimBundle(config *params.ChainConfig, bc *core.BlockChain, author *common.Address, gp *core.GasPool, statedb *state.StateDB, header *types.Header, b *SBundle, txIdx int, usedGas *uint64, cfg vm.Config, logs bool) (SimBundleResult, error) {
	res := NewSimBundleResult()

	currBlock := header.Number.Uint64()
	if currBlock < b.Inclusion.BlockNumber || currBlock > b.Inclusion.MaxBlockNumber {
		return res, errors.New("bundle not valid for current block")
	}

	var (
		coinbaseDelta  = new(uint256.Int)
		coinbaseBefore *uint256.Int
	)
	for _, el := range b.Body {
		coinbaseDelta.Set(common.U2560)
		// Coinbase 是builder的账户
		coinbaseBefore = statedb.GetBalance(header.Coinbase)

		if el.Tx != nil {
			statedb.SetTxContext(el.Tx.Hash(), txIdx)
			txIdx++
			receipt, result, err := ApplyTransactionWithResult(config, bc, author, gp, statedb, header, el.Tx, usedGas, cfg)
			if err != nil {
				return res, err
			}
			res.Revert = result.Revert()
			if result.Err != nil {
				res.ExecError = result.Err.Error()
			}
			if receipt.Status != types.ReceiptStatusSuccessful && !el.CanRevert {
				return res, errors.New("tx failed")
			}
			res.GasUsed += receipt.GasUsed
			if logs {
				res.BodyLogs = append(res.BodyLogs, SimBundleBodyLogs{TxLogs: receipt.Logs})
			}
		} else if el.Bundle != nil {
			innerRes, err := SimBundle(config, bc, author, gp, statedb, header, el.Bundle, txIdx, usedGas, cfg, logs)
			if err != nil {
				return res, err
			}
			// basically return first exec error if exists, helpful for single-tx sbundles
			if len(res.Revert) == 0 {
				res.Revert = innerRes.Revert
			}
			if len(res.ExecError) == 0 {
				res.ExecError = innerRes.ExecError
			}
			res.GasUsed += innerRes.GasUsed
			if logs {
				res.BodyLogs = append(res.BodyLogs, SimBundleBodyLogs{BundleLogs: innerRes.BodyLogs})
			}
		} else {
			return res, errors.New("invalid bundle body")
		}

		coinbaseAfter := statedb.GetBalance(header.Coinbase)
		coinbaseDelta.Set(coinbaseAfter)
		coinbaseDelta.Sub(coinbaseDelta, coinbaseBefore)

		res.TotalProfit.Add(res.TotalProfit, coinbaseDelta)
	}

	res.MevGasPrice.Div(res.TotalProfit, new(uint256.Int).SetUint64(res.GasUsed))
	return res, nil
}

func ApplyTransactionWithResult(config *params.ChainConfig, bc core.ChainContext, author *common.Address, gp *core.GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config) (*types.Receipt, *core.ExecutionResult, error) {
	msg, err := core.TransactionToMessage(tx, types.MakeSigner(config, header.Number, header.Time), header.BaseFee)
	if err != nil {
		return nil, nil, err
	}
	// Create a new context to be used in the EVM environment
	blockContext := core.NewEVMBlockContext(header, bc, author)
	//txContext := core.NewEVMTxContext(msg)
	vmenv := vm.NewEVM(blockContext, statedb, config, cfg)
	return applyTransactionWithResult(msg, config, gp, statedb, header.Number, header.Hash(), tx, usedGas, vmenv)
}

func applyTransactionWithResult(msg *core.Message, config *params.ChainConfig, gp *core.GasPool, statedb *state.StateDB, blockNumber *big.Int, blockHash common.Hash, tx *types.Transaction, usedGas *uint64, evm *vm.EVM) (*types.Receipt, *core.ExecutionResult, error) {
	// Create a new context to be used in the EVM environment.
	txContext := core.NewEVMTxContext(msg)

	//evm.Reset(txContext, statedb)
	evm.TxContext = txContext
	evm.StateDB = statedb

	// Apply the transaction to the current state (included in the env).
	result, err := core.ApplyMessage(evm, msg, gp)
	if err != nil {
		return nil, nil, err
	}

	// Update the state with pending changes.
	var root []byte
	if config.IsByzantium(blockNumber) {
		statedb.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(config.IsEIP158(blockNumber)).Bytes()
	}
	*usedGas += result.UsedGas

	// Create a new receipt for the transaction, storing the intermediate root and gas used
	// by the tx.
	receipt := &types.Receipt{Type: tx.Type(), PostState: root, CumulativeGasUsed: *usedGas}
	if result.Failed() {
		receipt.Status = types.ReceiptStatusFailed
	} else {
		receipt.Status = types.ReceiptStatusSuccessful
	}
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = result.UsedGas

	if tx.Type() == types.BlobTxType {
		receipt.BlobGasUsed = uint64(len(tx.BlobHashes()) * params.BlobTxBlobGasPerBlob)
		receipt.BlobGasPrice = evm.Context.BlobBaseFee
	}

	// If the transaction created a contract, store the creation address in the receipt.
	if msg.To == nil {
		receipt.ContractAddress = crypto.CreateAddress(evm.TxContext.Origin, tx.Nonce())
	}

	// Set the receipt logs and create the bloom filter.
	receipt.Logs = statedb.GetLogs(tx.Hash(), blockNumber.Uint64(), blockHash)
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	receipt.BlockHash = blockHash
	receipt.BlockNumber = blockNumber
	receipt.TransactionIndex = uint(statedb.TxIndex())
	return receipt, result, err
}
