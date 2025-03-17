package quant

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const ERC20ABI = `[{"constant":true,"inputs":[{"name":"owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[{"name":"owner","type":"address"},{"name":"spender","type":"address"}],"name":"allowance","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"name":"spender","type":"address"},{"name":"value","type":"uint256"}],"name":"approve","outputs":[{"name":"","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"}]`

func TestBackTrans(t *testing.T) {

	v3RouterAdress := common.HexToAddress("0x1b81d678ffb9c0263b24a97847620c99d213eb14") // 替换为实际的 spender 地址
	usdtAddress := common.HexToAddress("0x55d398326f99059fF775485246999027B3197955")
	bnbAdress := common.HexToAddress("0xBB4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c") // WBNB address
	userAdress := common.HexToAddress("0x8cB48323AcdAaA5f12CA0a1E5d59a97fd22CbeDA")

	client, err := ethclient.Dial("http://localhost:8545")
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}

	privateKey, err := crypto.HexToECDSA("dccb5a53608be54d42c0f8f8a258d667551134510ed9ed072fdef7f9d160ab94")
	if err != nil {
		log.Fatalf("Failed to load private key: %v", err)
	}

	nonce, err := client.PendingNonceAt(context.Background(), userAdress)
	if err != nil {
		log.Fatalf("Failed to get nonce: %v", err)
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatalf("Failed to get gas price: %v", err)
	}

	// 加载 ERC20 合约的 ABI
	parsedABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		log.Fatalf("Failed to parse ABI: %v", err)
	}
	// 根据代币地址创建合约实例
	tokenContract := bind.NewBoundContract(usdtAddress, parsedABI, client, client, client)

	// 查询授权余额
	var allowance = new(big.Int)
	var result []interface{}
	err = tokenContract.Call(&bind.CallOpts{Context: context.Background()}, &result, "allowance", userAdress, v3RouterAdress)
	if err != nil {
		log.Fatalf("Failed to query allowance: %v", err)
	}
	allowance = result[0].(*big.Int)
	if err != nil {
		log.Fatalf("Failed to query allowance: %v", err)
	}
	fmt.Printf("Current Token Allowance: %s\n", allowance.String())

	routerAbi, _ := ParseRouterAbi()   // Example fee tier
	amountIn := big.NewInt(1 * 1e18)   // Amount of USDT to swap (in wei)
	amountOutMinimum := big.NewInt(0)  // Minimum amount of BNB to receive (in wei)
	sqrtPriceLimitX96 := big.NewInt(0) // No price limit

	params := struct {
		TokenIn           common.Address
		TokenOut          common.Address
		Fee               *big.Int
		Recipient         common.Address
		Deadline          *big.Int
		AmountIn          *big.Int
		AmountOutMinimum  *big.Int
		SqrtPriceLimitX96 *big.Int
	}{
		TokenIn:           usdtAddress,
		TokenOut:          bnbAdress,
		Fee:               big.NewInt(500),
		Recipient:         userAdress,
		Deadline:          big.NewInt(time.Now().Add(time.Hour).Unix()), // 设置为当前时间加1小时
		AmountIn:          amountIn,
		AmountOutMinimum:  amountOutMinimum,
		SqrtPriceLimitX96: sqrtPriceLimitX96,
	}
	data, err := routerAbi.Pack("exactInputSingle", params)
	if err != nil {
		log.Fatalf("Failed to pack data: %v", err)
	}

	msg := ethereum.CallMsg{
		From:     userAdress,
		To:       &v3RouterAdress,
		Gas:      21000 * 10,
		GasPrice: gasPrice,
		Value:    big.NewInt(0),
		Data:     data,
	}

	// 使用 eth_call 模拟交易
	r, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		log.Fatalf("Failed to simulate transaction: %v", err)
	}

	// 检查返回结果
	if len(r) == 0 {
		log.Println("Transaction simulation successful, but no result returned")
	} else {
		log.Printf("Transaction simulation result: %x", result)
	}

	gasLimit, err := client.EstimateGas(context.Background(), msg)
	if err != nil {
		log.Fatalf("Failed to estimate gas: %v", err)
	}
	fmt.Printf("Gas Limit: %d\n", gasLimit)
	/////////////////////////////////////////////////////////////////////////////////////
	tx := types.NewTransaction(nonce, v3RouterAdress, big.NewInt(0), 300000, gasPrice, data)

	// 签名交易
	chainID := big.NewInt(56)
	if err != nil {
		log.Fatalf("Failed to get network ID: %v", err)
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		log.Fatalf("Failed to sign transaction: %v", err)
	}

	// 发送交易
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		log.Fatalf("Failed to send transaction: %v", err)
	}

	fmt.Printf("Transaction sent: %s\n", signedTx.Hash().Hex())

	data, err = signedTx.MarshalBinary()
	if err != nil {
		log.Fatalf("Failed to marshal transaction: %v", err)
	}
	fmt.Printf("Transaction Data: %x\n", data)
}
func TestParseTrans(t *testing.T) {
	// 连接到以太坊客户端
	client, err := ethclient.Dial("http://bsc-dataseed.binance.org/")
	if err != nil {
		t.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}

	// 交易哈希
	txHash := common.HexToHash("0xd75b9a83974dfa75b608dc7727c75f36215a43ae0c3efd1df082dde878f73b02")
	routerAbi, _ := ParseRouterAbi()

	// 查询交易
	tx, _, err := client.TransactionByHash(context.Background(), txHash)
	if err != nil {
		t.Fatalf("Failed to get transaction by hash: %v", err)
	}
	// 获取交易数据
	data := tx.Data()

	// 解析方法名和参数
	method, err := routerAbi.MethodById(data[:4])
	if err != nil {
		t.Fatalf("Failed to get method by ID: %v", err)
	}

	// 跳过函数选择器4字节
	inputs, err := method.Inputs.Unpack(data[4:])
	if err != nil {
		log.Fatal(err)
	}

	// 展开解码后的参数
	params := inputs[0].(struct {
		TokenIn           common.Address
		TokenOut          common.Address
		Fee               uint32
		Recipient         common.Address
		Deadline          *big.Int
		AmountIn          *big.Int
		AmountOutMinimum  *big.Int
		SqrtPriceLimitX96 *big.Int
	})

	fmt.Println("tokenIn:", params.TokenIn.Hex())
	fmt.Println("tokenOut:", params.TokenOut.Hex())
	fmt.Println("fee:", params.Fee)
	fmt.Println("recipient:", params.Recipient.Hex())
	fmt.Println("deadline:", params.Deadline.String())
	fmt.Println("amountIn:", params.AmountIn.String())
	fmt.Println("amountOutMinimum:", params.AmountOutMinimum.String())
	fmt.Println("sqrtPriceLimitX96:", params.SqrtPriceLimitX96.String())
}
