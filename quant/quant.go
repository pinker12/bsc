package quant

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/internal/ethapi"
)

const poolAddress = "0x36696169C63e42cd08ce11f5deeBbCeBae652050"

type Quant struct {
	abi            *abi.ABI
	apiBackend     ethapi.Backend
	eth            *eth.Ethereum
	txpoolApi      *ethapi.TxPoolAPI
	transactionApi *ethapi.TransactionAPI
	blockChainApi  *ethapi.BlockChainAPI
	Contract       *bind.BoundContract
	httpClient     *ethclient.Client
	storage        *Storage
	strategy       *Strategy
}

func NewQuant(apibackend ethapi.Backend, eth *eth.Ethereum) *Quant {
	nonceLock := new(ethapi.AddrLocker)
	txApi := ethapi.NewTxPoolAPI(apibackend)
	transactionApi := ethapi.NewTransactionAPI(apibackend, nonceLock)
	blockChainApi := ethapi.NewBlockChainAPI(apibackend)

	strategy := NewStrategy(apibackend, eth)

	client, err := ethclient.Dial("http://localhost:8545")
	if err != nil {
		log.Printf("Failed to init ethclient: %v", err)
		return nil
	}

	abi, _ := ParseAbi()
	bc := bind.NewBoundContract(common.HexToAddress(poolAddress), *abi, client, client, client)

	return &Quant{
		apiBackend:     apibackend,
		eth:            eth,
		txpoolApi:      txApi,
		transactionApi: transactionApi,
		httpClient:     client,
		Contract:       bc,
		storage:        NewStorage(),
		blockChainApi:  blockChainApi,
		abi:            abi,
		strategy:       strategy,
	}
}

func ParseAbi() (*abi.ABI, error) {
	abiFile, err := os.Open("/home/allenmwang/remote/test/bsc/quant/abi/pancakeV3PoolABI.json")
	if err != nil {
		return nil, err
	}
	defer abiFile.Close()

	abiBytes, err := io.ReadAll(abiFile)
	if err != nil {
		return nil, err
	}

	parsedABI, err := abi.JSON(strings.NewReader(string(abiBytes)))
	if err != nil {
		return nil, err
	}
	return &parsedABI, nil
}

func ParseRouterAbi() (*abi.ABI, error) {
	abiFile, err := os.Open("/home/allenmwang/remote/test/bsc/quant/abi/v3swapRouterABI.json")
	if err != nil {
		return nil, err
	}
	defer abiFile.Close()

	abiBytes, err := io.ReadAll(abiFile)
	if err != nil {
		return nil, err
	}

	parsedABI, err := abi.JSON(strings.NewReader(string(abiBytes)))
	if err != nil {
		return nil, err
	}
	return &parsedABI, nil
}

func (q *Quant) Loop() {

	targetAddresses := map[string]struct{}{
		"0xDa77c035e4d5A748b4aB6674327Fa446F17098A2": {},
		"0x000000000008D5760657dD664c7096897E1AE801": {},
		"0x802b65b5d9016621E66003aeD0b16615093f328b": {},
		"0x1A0A18AC4BECDDbd6389559687d1A73d8927E416": {},
		"0x0000000055ECa968153aeFfa4e421f9cd2680f01": {},
		"0x17Cd8E8D4c64aa7f2a8Db6947b330885DA56b833": {},
		"0x32564234dF8961ae1d640bE2CBA4aEab54151551": {},
		"0x9333C74BDd1E118634fE5664ACA7a9710b108Bab": {},
		"0x36EbED00Ae87c55e1036a8af96dd96380958F4C3": {},
		"0x773ae23983e1e9720BAfe6E214971Ff762D9758D": {},
		"0x0000000040Ac064de24cBC1cB9FCbcbC033eE5B6": {},
		"0x5D80acAf6C75F2DEaDf935f21Bd09C9cc4d61d9B": {},
		"0x69460570c93f9DE5E2edbC3052bf10125f0Ca22d": {},
		"0x3e704f2bC43c6408a5Cf0638a977b82bFB20d748": {},
		"0x00000047bB99ea4D791bb749D970DE71EE0b1A34": {},
		"0xce16F69375520ab01377ce7B88f5BA8C48F8D666": {},
		"0x0000000256Cdb6d26cF9FD79229976b0fEaECdF4": {},
		"0xF552951F9D5f83E2D94D87FcA14545CD33E93d2C": {},
		"0x013bb8a204499523ddF717e0aBAA14E6dC849060": {},
		"0xe82c715e37f2f2E190dD2cA86Fb796CAFaF0bEFf": {},
	}

	swapEventSignature := []byte("Swap(address,address,int256,int256,uint160,uint128,int24,uint128,uint128)")
	swapEventHash := crypto.Keccak256Hash(swapEventSignature)

	// pending transaction
	transaction := make(chan []*types.Transaction)
	// swap Event
	logs := make(chan []*types.Log)

	crit := ethereum.FilterQuery{
		Addresses: []common.Address{common.HexToAddress(poolAddress)},
		Topics:    [][]common.Hash{{swapEventHash}},
	}

	eventSystem := filters.NewEventSystem(filters.NewFilterSystem(q.apiBackend, filters.Config{}))
	eventSystem.SubscribeLogs(crit, logs)
	eventSystem.SubscribePendingTxs(transaction)
	for {
		select {
		case txs := <-transaction:
			for _, tx := range txs {
				if tx.To() != nil {
					toAddress := tx.To().Hex()
					fmt.Printf("To: %s\n", toAddress)
					if _, found := targetAddresses[toAddress]; found {
						// 保存到 Elasticsearch
						timestamp := time.Now().Format(time.RFC3339)
						txInfo := map[string]interface{}{
							"hash":      tx.Hash().Hex(),
							"to":        toAddress,
							"value":     tx.Value().String(),
							"gas":       tx.Gas(),
							"gasPrice":  tx.GasPrice().String(),
							"nonce":     tx.Nonce(),
							"timestamp": timestamp,
						}
						_, err := q.storage.Put(txInfo, "transactions")
						if err != nil {
							log.Printf("Failed to save transaction to Elasticsearch: %v", err)
						}
						fmt.Printf("Hash: %s\n", tx.Hash().Hex())
						fmt.Printf("Value: %s\n", tx.Value().String())
						fmt.Printf("Gas: %d\n", tx.Gas())
						fmt.Printf("Gas Price: %s\n", tx.GasPrice().String())
						fmt.Printf("Nonce: %d\n", tx.Nonce())
						fmt.Println("===================================")
						q.strategy.Try(tx)
					}
				}
			}
		case logs := <-logs:
			for _, log := range logs {
				q.handleSwapEvent(*log)
			}
		}
	}
}

func (q *Quant) handleSwapEvent(vLog types.Log) error {
	fmt.Println("Swap event detected:################")
	// Parse event data
	event := struct {
		Sender             common.Address
		Recipient          common.Address
		Amount0            *big.Int
		Amount1            *big.Int
		SqrtPriceX96       *big.Int
		Liquidity          *big.Int
		Tick               *big.Int
		ProtocolFeesToken0 *big.Int
		ProtocolFeesToken1 *big.Int
	}{}

	err := q.Contract.UnpackLog(&event, "Swap", vLog)
	if err != nil {
		log.Printf("Failed to unpack log: %v", err)
		return err
	}

	// Convert amounts to readable format
	readableAmount0 := new(big.Float).Quo(new(big.Float).SetInt(event.Amount0), big.NewFloat(math.Pow10(18))) // Assuming USDT has 6 decimals
	readableAmount1 := new(big.Float).Quo(new(big.Float).SetInt(event.Amount1), big.NewFloat(math.Pow10(18))) // Assuming BNB has 18 decimals

	protocolFeeToken0 := new(big.Float).Quo(new(big.Float).SetInt(event.ProtocolFeesToken0), big.NewFloat(math.Pow10(18)))
	protocolFeeToken1 := new(big.Float).Quo(new(big.Float).SetInt(event.ProtocolFeesToken1), big.NewFloat(math.Pow10(18)))

	// Calculate fee
	var fee float64
	var accuracy big.Accuracy
	if event.Amount0.Cmp(big.NewInt(0)) > 0 {
		fee, accuracy = new(big.Float).Quo(protocolFeeToken0, readableAmount0).Float64()
	} else {
		fee, accuracy = new(big.Float).Quo(protocolFeeToken1, readableAmount1).Float64()
	}
	fmt.Println("fee:", fee, accuracy)

	// Get transaction receipt
	receipt, err := q.transactionApi.GetTransactionReceipt(context.Background(), vLog.TxHash)
	txHash := vLog.TxHash.Hex()
	fmt.Println(txHash)
	if err != nil {
		log.Printf("Failed to get transaction receipt: %v", err)
		return err
	}

	// Extract gasUsed and effectiveGasPrice from receipt
	gasUsed, ok := receipt["gasUsed"].(hexutil.Uint64)
	if !ok {
		return fmt.Errorf("invalid gasUsed type")
	}

	effectiveGasPrice, ok := receipt["effectiveGasPrice"].(*hexutil.Big)
	if !ok {
		return fmt.Errorf("invalid effectiveGasPrice type")
	}

	// Calculate gasFee
	gasFee := new(big.Int).Mul(new(big.Int).SetUint64(uint64(gasUsed)), effectiveGasPrice.ToInt())
	// Get block timestamp
	block, err := q.blockChainApi.GetBlockByHash(context.Background(), receipt["blockHash"].(common.Hash), false)
	if err != nil {
		log.Printf("Failed to get block: %v", err)
		return err
	}
	timestamp := time.Unix(int64(block["timestamp"].(hexutil.Uint64)), 0).Format(time.RFC3339)

	//timestamp := block["timestamp"].(hexutil.Uint64)

	var toAddress string
	if receipt["to"].(*common.Address) != nil {
		toAddress = receipt["to"].(*common.Address).Hex()
	} else {
		toAddress = "Contract Creation"
	}

	swapEvent := SwapEvent{
		Pair:             poolAddress,
		Token0:           "USDT",
		Token1:           "BNB",
		TransactionInfo:  TBase{TransactionHash: vLog.TxHash.Hex(), From: receipt["from"].(common.Address).String(), To: toAddress},
		Sender:           event.Sender.Hex(),
		Recipient:        event.Recipient.Hex(),
		Tick:             event.Tick.Int64(),
		Amount0:          func() float64 { f, _ := readableAmount0.Float64(); return f }(),
		Amount0Origin:    event.Amount0.Int64(),
		Amount1:          func() float64 { f, _ := readableAmount1.Float64(); return f }(),
		Amount1Origin:    event.Amount1.Int64(),
		SqrtPriceX96:     event.SqrtPriceX96.Int64(),
		Liquidity:        event.Liquidity.Int64(),
		FeeAmount0:       func() float64 { f, _ := protocolFeeToken0.Float64(); return f }(),
		FeeAmount0Origin: event.ProtocolFeesToken0.Int64(),
		FeeAmount1:       func() float64 { f, _ := protocolFeeToken1.Float64(); return f }(),
		FeeAmount1Origin: event.ProtocolFeesToken1.Int64(),
		Fee:              fee,
		GasFee:           gasFee.Int64(),
		GasUsed:          uint64(gasUsed),
		GasPrice:         effectiveGasPrice.ToInt().Int64(),
		Timestamp:        timestamp,
	}
	// fmt.Printf(`Swap event details:
	// Pair: %s
	// Token0: %s
	// Token1: %s
	// Transaction Hash: %s
	// Transaction from: %s
	// Transaction to: %s
	// Sender: %s
	// Recipient: %s
	// Tick: %d
	// Amount0: %f USDT
	// Amount0 origin: %d USDT
	// Amount1: %f BNB
	// Amount1 origin: %d BNB
	// SqrtPriceX96: %d
	// Liquidity: %d
	// Fee Amount0: %f USDT
	// Fee Amount0 origin: %d USDT
	// Fee Amount1: %f BNB
	// Fee Amount1 origin: %d BNB
	// Fee: %f
	// GasFee: %d Gwei
	// GasUsed: %d
	// GasPrice: %d Gwei
	// Timestamp: %s
	// `,
	// 	swapEvent.Pair,
	// 	swapEvent.Token0,
	// 	swapEvent.Token1,
	// 	swapEvent.TransactionInfo.TransactionHash,
	// 	swapEvent.TransactionInfo.From,
	// 	swapEvent.TransactionInfo.To,
	// 	swapEvent.Sender,
	// 	swapEvent.Recipient,
	// 	swapEvent.Tick,
	// 	swapEvent.Amount0,
	// 	swapEvent.Amount0Origin,
	// 	swapEvent.Amount1,
	// 	swapEvent.Amount1Origin,
	// 	swapEvent.SqrtPriceX96,
	// 	swapEvent.Liquidity,
	// 	swapEvent.FeeAmount0,
	// 	swapEvent.FeeAmount0Origin,
	// 	swapEvent.FeeAmount1,
	// 	swapEvent.FeeAmount1Origin,
	// 	swapEvent.Fee,
	// 	swapEvent.GasFee,
	// 	swapEvent.GasUsed,
	// 	swapEvent.GasPrice,
	// 	swapEvent.Timestamp,
	// )

	// Save to Elasticsearch
	err = q.storage.BulkPut([]interface{}{swapEvent}, "swap_events")
	if err != nil {
		log.Printf("Failed to save swap event to Elasticsearch: %v", err)
		return err
	}

	return nil
}
