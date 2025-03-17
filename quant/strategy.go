package quant

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/internal/ethapi"
)

var v3RouterAdress common.Address
var usdtAddress common.Address
var bnbAdress common.Address
var userAdress common.Address
var bloxroute common.Address

func init() {
	v3RouterAdress = common.HexToAddress("0x1b81d678ffb9c0263b24a97847620c99d213eb14") // 替换为实际的 spender 地址
	usdtAddress = common.HexToAddress("0x55d398326f99059fF775485246999027B3197955")
	usdtAddress = common.HexToAddress("0x55d398326f99059fF775485246999027B3197955")
	bnbAdress = common.HexToAddress("0xBB4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c") // WBNB address
	userAdress = common.HexToAddress("0x8cB48323AcdAaA5f12CA0a1E5d59a97fd22CbeDA")
	bloxroute = common.HexToAddress("0x74c5F8C6ffe41AD4789602BDB9a48E6Cad623520")
}

type Strategy struct {
	poolAbi    *abi.ABI
	routerAbi  *abi.ABI
	txs        chan *types.Transaction
	apiBackend ethapi.Backend
	eth        *eth.Ethereum
}

func NewStrategy(apibackend ethapi.Backend, eth *eth.Ethereum) *Strategy {
	txs := make(chan *types.Transaction, 100)
	abi, _ := ParseAbi()
	routerAbi, _ := ParseRouterAbi()
	return &Strategy{
		txs:        txs,
		poolAbi:    abi,
		routerAbi:  routerAbi,
		apiBackend: apibackend,
		eth:        eth,
	}
}

func (s *Strategy) PutTx(tx *types.Transaction) {
	s.txs <- tx
}

// 定义 Swap 事件的结构体
type Event struct {
	Sender             common.Address
	Recipient          common.Address
	Amount0            *big.Int
	Amount1            *big.Int
	SqrtPriceX96       *big.Int
	Liquidity          *big.Int
	Tick               *big.Int
	ProtocolFeesToken0 *big.Int
	ProtocolFeesToken1 *big.Int
}

type ExactInputParams struct {
	TokenIn           common.Address
	TokenOut          common.Address
	Fee               *big.Int
	Recipient         common.Address
	Deadline          *big.Int
	AmountIn          *big.Int
	AmountOutMinimum  *big.Int
	SqrtPriceLimitX96 *big.Int
}

type ExactOutPutParams struct {
	TokenIn           common.Address
	TokenOut          common.Address
	Fee               *big.Int
	Recipient         common.Address
	Deadline          *big.Int
	AmountOut         *big.Int
	AmountOutMinimum  *big.Int
	SqrtPriceLimitX96 *big.Int
}

func (s *Strategy) Try(tx *types.Transaction) (*types.Receipt, error) {
	eth := s.eth

	chainConfig := eth.BlockChain().Config()
	chain := eth.BlockChain()
	statedb, err := eth.BlockChain().State()
	if err != nil {
		return nil, err
	}
	gasPool := new(core.GasPool).AddGas(chain.GasLimit())
	hash := tx.Hash().Hex()

	fmt.Println(hash)
	// Create a new context to be used in the EVM environment
	//var processor []core.ReceiptProcessor
	header := chain.CurrentHeader()
	blockContext := core.NewEVMBlockContext(header, ethapi.NewChainContext(context.Background(), s.apiBackend), nil)
	vmenv := vm.NewEVM(blockContext, statedb, chainConfig, vm.Config{})

	snapshot := statedb.Snapshot()
	var zero uint64 = 0
	// Start executing the transaction
	statedb.SetTxContext(tx.Hash(), 1)
	receipt, err := core.ApplyTransactionPersonal(vmenv, gasPool, statedb, chain.CurrentHeader(), tx, &zero)
	if err != nil {
		return nil, err
	}
	statedb.RevertToSnapshot(snapshot)

	for _, vLog := range receipt.Logs {
		// 检查事件的签名是否匹配
		if vLog.Topics[0] == common.HexToHash("0x19b47279256b2a23a1665c810c8d55a1758940ee09377d4f8d26497a3577dc83") {
			// 解析事件
			var event Event
			err := s.poolAbi.UnpackIntoInterface(&event, "Swap", vLog.Data)
			if err != nil {
				log.Printf("failed to unpack log: %v", err)
				continue
			}

			// 定义 100 * 1e18
			threshold := new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18))

			// 判断 event.Amount0 是否大于 100 * 1e18
			if event.Amount1.Cmp(threshold) > 0 {
				fmt.Println("Amount0 is greater than 100 * 1e18")

			}
			var txs []*types.Transaction
			var txsData []string

			// front tx
			frontTx, frontData, err := s.FrontTrans(big.NewInt(1e18), 3)
			txs = append(txs, frontTx)
			txsData = append(txsData, frontData)
			// victim tx
			victimTx := tx
			victimData, err := s.GetData(victimTx)
			if err != nil {
				log.Fatalf("Failed to marshal transaction: %v", err)
			}
			txs = append(txs, victimTx)
			txsData = append(txsData, victimData)

			// back tx
			backTx, backData, err := s.BackTrans(big.NewInt(1e18), 4)
			txs = append(txs, backTx)
			txsData = append(txsData, backData)

			// bribe tx
			bribeTx, bribeData, err := s.BloxRouteTx(5, big.NewInt(1e15), big.NewInt(1e9))
			txs = append(txs, bribeTx)
			txsData = append(txsData, bribeData)

			// 打印事件详情
			fmt.Printf("Swap Event: %+v\n", event)
		}
	}
	return receipt, nil
}

func (s *Strategy) SimulateTxs(tx []*types.Transaction) (*types.Receipt, error) {

}

func (s *Strategy) CtreateExactInputTx(tokenIn common.Address, tokenOut common.Address,
	amountIn *big.Int, nonce uint64) (*types.Transaction, error) {

	params := ExactInputParams{
		TokenIn:           tokenIn,
		TokenOut:          tokenOut,
		Fee:               big.NewInt(500),
		Recipient:         userAdress,
		Deadline:          big.NewInt(time.Now().Add(time.Hour).Unix()), // 设置为当前时间加1小时
		AmountIn:          amountIn,
		AmountOutMinimum:  big.NewInt(0),
		SqrtPriceLimitX96: big.NewInt(0),
	}

	data, err := s.routerAbi.Pack("exactInputSingle", params)
	if err != nil {
		log.Fatalf("Failed to pack data: %v", err)
	}

	// {
	// 	Nonce:    nonce,
	// 	To:       &to,
	// 	Value:    amount,
	// 	Gas:      gasLimit,
	// 	GasPrice: gasPrice,
	// 	Data:     data,
	// }
	gasPrice := big.NewInt(1e9) //  1 Gwei
	tx := types.NewTransaction(nonce, v3RouterAdress, big.NewInt(0), 300000, gasPrice, data)
	return tx, nil

}

func (s *Strategy) CtreateExactOutPutTx(tokenIn common.Address, tokenOut common.Address,
	amountOut *big.Int, nonce uint64) (*types.Transaction, error) {

	params := ExactInputParams{
		TokenIn:           tokenIn,
		TokenOut:          tokenOut,
		Fee:               big.NewInt(500),
		Recipient:         userAdress,
		Deadline:          big.NewInt(time.Now().Add(time.Hour).Unix()), // 设置为当前时间加1小时
		AmountIn:          amountOut,
		AmountOutMinimum:  big.NewInt(0),
		SqrtPriceLimitX96: big.NewInt(0),
	}

	data, err := s.routerAbi.Pack("exactOutputSingle", params)
	if err != nil {
		log.Fatalf("Failed to pack data: %v", err)
	}
	gasPrice := big.NewInt(1e9) //  1 Gwei
	tx := types.NewTransaction(nonce, v3RouterAdress, big.NewInt(0), 300000, gasPrice, data)
	return tx, nil

}

// 买入bnb, 使用ExactOut，固定买入一个bnb
func (s *Strategy) FrontTrans(bnb *big.Int, nonce uint64) (*types.Transaction, string, error) {
	tx, err := s.CtreateExactOutPutTx(usdtAddress, bnbAdress, bnb, nonce)
	if err != nil {
		return nil, "", err
	}
	rawTxHex, err := s.SignTx(tx)
	return tx, rawTxHex, err
}

// 卖出bnb，使用ExactInput，固定卖出一个bnb
func (s *Strategy) BackTrans(bnb *big.Int, nonce uint64) (*types.Transaction, string, error) {
	tx, err := s.CtreateExactInputTx(bnbAdress, usdtAddress, bnb, nonce)
	if err != nil {
		return nil, "", err
	}
	rawTxHex, err := s.SignTx(tx)
	return tx, rawTxHex, err
}

func (s *Strategy) BloxRouteTx(nonce uint64, amount *big.Int, gasPrice *big.Int) (*types.Transaction, string, error) {
	tx := types.NewTransaction(nonce, v3RouterAdress, amount, 300000, gasPrice, nil)
	rawTxHex, err := s.SignTx(tx)
	return tx, rawTxHex, err
}

func (s *Strategy) SignTx(tx *types.Transaction) (string, error) {
	// 签名交易
	chainID := big.NewInt(56)
	privateKey, err := crypto.HexToECDSA("your_private_key_here")
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		log.Fatalf("Failed to sign transaction: %v", err)
	}

	txData, err := s.GetData(signedTx)
	if err != nil {
		log.Fatalf("Failed to sign transaction: %v", err)
	}
	return txData, nil
}

func (s *Strategy) GetData(tx *types.Transaction) (string, error) {
	data, err := tx.MarshalBinary()
	if err != nil {
		return "", err
	}
	return common.Bytes2Hex(data), nil
}
