package quant

import (
	"encoding/json"
	"fmt"
	"log"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

func getTransactionTraces(client *rpc.Client, txHash common.Hash) ([]map[string]interface{}, error) {
	var result []map[string]interface{}
	err := client.Call(&result, "debug_traceTransaction", txHash, map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction traces: %v", err)
	}
	return result, nil
}

func TestGetTransactionTraces(t *testing.T) {
	// 连接本地 BSC 节点
	rpcClient, err := rpc.Dial("http://localhost:8545")
	if err != nil {
		t.Fatalf("Failed to connect to the RPC client: %v", err)
	}

	// 示例交易哈希
	txHash := common.HexToHash("0x357e144832d50293ff069ea78df38d46d8bfcc2f7e4bf1c756e0443ca3e89d33")

	// 获取交易的 traces,是运行的字节码
	// {"pc":2895,"op":"SWAP2","gas":189411,"gasCost":3,"depth":1,"stack":["0xd9e301b7","0x39d","0x104","0x13c9","0x0"]},{"pc":2896,"op":"PUSH1","gas":189408,"gasCost":3,"depth":1,"stack":["0xd9e301b7","0x39d","0x0","0x13c9","0x104"]},{"pc":2898,"op":"CALLDATALOAD","gas":189405,"gasCost":3,"depth":1,"stack":["0xd9e301b7","0x39d","0x0","0x13c9","0x104","0x24"]},{"pc":2899,"op":"PUSH2","gas":189402,"gasCost":3,"depth":1,"stack":["0xd9e301b7","0x39d","0x0","0x13c9","0x104","0x36696169c63e42cd08ce11f5deebbcebae652050"]},{"pc":2902,"op":"DUP2","gas":189399,"gasCost":3,"depth":1,"stack":["0xd9e301b7","0x39d","0x0","0x13c9","0x104","0x36696169c63e42cd08ce11f5deebbcebae652050","0xb5b"]},{"pc":2903,"op":"PUSH2","gas":189396,"gasCost":3,"depth":1,"stack":["0xd9e301b7","0x39d","0x0","0x13c9","0x104","0x36696169c63e42cd08ce11f5deebbcebae652050","0xb5b","0x36696169c63e42cd08ce11f5deebbcebae652050"]},{"pc":2906,"op":"JUMP","gas":189393,"gasCost":8,"depth":1,"stack":["0xd9e301b7","0x39d","0x0","0x13c9","0x104","0x36696169c63e42cd08ce11f5deebbcebae652050","0xb5b","0x36696169c63e42cd08ce11f5deebbcebae652050","0x290"]},{"pc":656,"op":"JUMPDEST","gas":189385,"gasCost":1,"depth":1,"stack":["0xd9e301b7","0x39d","0x0","0x13c9","0x104","0x36696169c63e42cd08ce11f5deebbcebae652050","0xb5b","0x36696169c63e42cd08ce11f5deebbcebae652050"]},{"pc":657,"op":"PUSH1","gas":189384,"gasCost":3,"depth":1,"stack":["0xd9e301b7","0x39d","0x0","0x13c9","0x104","0x36696169c63e42cd08ce11f5deebbcebae652050","0xb5b","0x36696169c63e42cd08ce11f5deebbcebae652050"]},{"pc":659,"op":"PUSH1","gas":189381,"gasCost":3,"depth":1,"stack":["0xd9e301b7","0x39d","0x0","0x13c9","0x104","0x36696169c63e42cd08ce11f5deebbcebae652050","0xb5b","0x36696169c63e42cd08ce11f5deebbcebae652050","0x1"]},{"pc":661,"op":"PUSH1","gas":189378,"gasCost":3,"depth":1,"stack":
	traces, err := getTransactionTraces(rpcClient, txHash)
	if err != nil {
		t.Fatalf("Failed to get transaction traces: %v", err)
	}

	// 打印 traces
	tracesJSON, _ := json.MarshalIndent(traces, "", "  ")
	log.Printf("Transaction Traces: %s\n", tracesJSON)

	// 断言 traces 不为空
	if len(traces) == 0 {
		t.Fatalf("Expected non-empty traces, got empty")
	}
}
