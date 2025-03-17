package quant

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Builder struct {
	Authorization string
}

func NewBuilder() *Builder {
	return &Builder{
		Authorization: "YWRjOTAwNDMtYjVhNy00ZGNiLWIyOTEtZWM0NWViNzVjOTE5Ojg4YmQ5MGNhNGFiZjIwYzgxN2MzODQ4MDRhMjJjNDg2",
	}
}

func (b *Builder) RequestBscMevValidators() ([]string, error) {
	payload := CommonRequest{
		ID:     "1",
		Method: "bsc_mev_validators",
		Params: BscMevParams{
			BlockchainNetwork: "BSC-Mainnet",
		},
	}

	result, err := RequestBscMev(b.Authorization, payload)
	if err != nil {
		return nil, err
	}

	var response BscMevResponse
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, err
	}

	return response.Result.Validators, nil
}

func (b *Builder) RequestBlxrSubmitBundle() error {
	payload := CommonRequest{
		ID:     "1",
		Method: "blxr_submit_bundle",
		Params: BlxrSubmitBundleParams{
			Transaction: []string{
				"0x1",
				"0x2",
			},
			BlockchainNetwork: "bsc",
			BlockNumber:       "123",
		},
	}

	_, err := RequestBscMev(b.Authorization, payload)
	return err
}

func (b *Builder) RequestBlxrSimulateBundle() error {
	payload := CommonRequest{
		ID:     "1",
		Method: "blxr_simulate_bundle",
		Params: BlxrSimulateBundleParams{
			Transaction:       []string{"0x1", "0x2"},
			BlockNumber:       "123",
			StateBlockNumber:  "latest",
			Timestamp:         1234567890,
			BlockchainNetwork: "bsc",
		},
	}

	_, err := RequestBscMev(b.Authorization, payload)
	return err
}

type BlxrSimulateBundleParams struct {
	Transaction       []string `json:"transaction"`        // 一组不包含 0x 前缀的原始交易字节
	BlockNumber       string   `json:"block_number"`       // 以十六进制表示的未来区块编号
	StateBlockNumber  string   `json:"state_block_number"` // 指定要模拟的状态区块，可为 "latest" 等
	Timestamp         int64    `json:"timestamp"`          // 用于模拟的时间戳，unix时间格式
	BlockchainNetwork string   `json:"blockchain_network"` // 区块链网络名称，例如 Mainnet
}

type BlxrSubmitBundleParams struct {
	Transaction       []string `json:"transaction"`                // 一组不包含 0x 前缀的原始交易字节，以逗号分隔。
	BlockchainNetwork string   `json:"blockchain_network"`         // 必须是 BSC-Mainnet。
	BlockNumber       string   `json:"block_number"`               // 以十六进制表示的未来区块编号。
	MinTimestamp      *int64   `json:"min_timestamp,omitempty"`    // [可选] 最小时间戳（unix 时间），默认为 None。
	MaxTimestamp      *int64   `json:"max_timestamp,omitempty"`    // [可选] 最大时间戳（unix 时间），默认为 None。
	RevertingHashes   []string `json:"reverting_hashes,omitempty"` // [可选] 允许回滚的交易哈希列表，默认空列表，若任意交易回滚则排除整个 bundle。
	DroppingHashes    []string `json:"dropping_hashes,omitempty"`  // [可选] 在无效时可从 bundle 中移除的交易哈希列表，默认空列表。
	BlocksCount       int      `json:"blocks_count,omitempty"`     // [可选，默认: 1] 指定 bundle 可用的后续区块数量，最大 20。
	MevBuilders       struct {
		All string `json:"all"`
	} `json:"mev_builders,omitempty"` // [可选，默认: all] 指定接收此 bundle 的 MEV builder。bloxroute 始终可用。
	AvoidMixedBundles bool `json:"avoid_mixed_bundles,omitempty"` // [可选，默认: false] 若为 false，允许与其他 bundle 或交易混合。
	EndOfBlock        bool `json:"end_of_block,omitempty"`        // [可选，默认: false] 若为 true，将此 bundle 尽量放在区块末端。
}

type CommonRequest struct {
	ID     string      `json:"id"`     // 请求的唯一标识符
	Method string      `json:"method"` // 请求的方法名称，如 "submit_bundle"
	Params interface{} `json:"params"` // 包含提交 bundle 的各项参数
}

type BscMevParams struct {
	BlockchainNetwork string `json:"blockchain_network"`
}

type BscMevResponse struct {
	ID     string `json:"id"`
	Result struct {
		Validators []string `json:"validators"`
	} `json:"result"`
	JSONRPC string `json:"jsonrpc"`
}

// 直接使用 json.RawMessage 作为请求返回值
func RequestBscMev(authorizationHeader string, payload interface{}) (json.RawMessage, error) {
	url := "https://mev.api.blxrbdn.com"

	reqBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authorizationHeader)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return bodyBytes, nil
}
