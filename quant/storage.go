package quant

import (
	"context"
	"fmt"
	"log"

	"github.com/olivere/elastic"
)

type SwapEvent struct {
	Pair             string  `json:"pair"`
	Token0           string  `json:"token0"`
	Token1           string  `json:"token1"`
	TransactionInfo  TBase   `json:"transaction_info"`
	Sender           string  `json:"sender"`
	Recipient        string  `json:"recipient"`
	Tick             int64   `json:"tick"`
	Amount0          float64 `json:"amount0"`
	Amount0Origin    int64   `json:"amount0_origin"`
	Amount1          float64 `json:"amount1"`
	Amount1Origin    int64   `json:"amount1_origin"`
	SqrtPriceX96     int64   `json:"sqrt_price_x96"`
	Liquidity        int64   `json:"liquidity"`
	FeeAmount0       float64 `json:"fee_amount0"`
	FeeAmount0Origin int64   `json:"fee_amount0_origin"`
	FeeAmount1       float64 `json:"fee_amount1"`
	FeeAmount1Origin int64   `json:"fee_amount1_origin"`
	Fee              float64 `json:"fee"`
	GasFee           int64   `json:"gas_fee"`
	GasUsed          uint64  `json:"gas_used"`
	GasPrice         int64   `json:"gas_price"`
	Timestamp        string  `json:"timestamp"` // 添加时间戳字段
}

type TBase struct {
	TransactionHash string `json:"transaction_hash"`
	From            string `json:"from"`
	To              string `json:"to"`
}

type Storage struct {
	esClient *elastic.Client
}

func NewStorage() *Storage {
	client, err := elastic.NewClient(elastic.SetURL("http://elastic:ByGXK*4725yhAP@9.135.244.180:9200"),
		elastic.SetSniff(false), elastic.SetHealthcheck(false))
	if err != nil {
		panic(err)
	}
	return &Storage{
		esClient: client,
	}
}

func (s *Storage) Put(meta interface{}, index string) (string, error) {
	put1, err := s.esClient.Index().
		Index(index).
		Type("doc").
		BodyJson(meta).
		Do(context.Background())
	if err != nil {
		return "", err
	}

	return put1.Id, nil
}

func (s *Storage) BulkPut(docs []interface{}, index string) error {
	bulkRequest := s.esClient.Bulk()
	for _, doc := range docs {
		req := elastic.NewBulkIndexRequest().Index(index).Doc(doc)
		bulkRequest = bulkRequest.Add(req)
	}

	bulkResponse, err := bulkRequest.Do(context.Background())
	if err != nil {
		return err
	}

	if bulkResponse.Errors {
		for _, item := range bulkResponse.Failed() {
			log.Printf("Failed to index document %s: %s", item.Id, item.Error.Reason)
		}
		return fmt.Errorf("bulk indexing failed")
	}

	return nil
}
