package types

import "sync"

/////////////////////Types//////////////////////////

type RespNyksBlockWithTx struct {
	Block_id BlockID
	Txs      []Transaction
	Block    Block
	Code     int8
}

type RespNyksBlock struct {
	Block_id BlockID
	Block    Block
}

type Block struct {
	Header BlockHeader
	Data   Blockdata
}

type Blockdata struct {
	Txs []string
}

type BlockHeader struct {
	Height string
}

type BlockID struct {
	Hash string
}

type TxDetailsResp struct {
	Tx          Transaction
	Tx_response TxResponse
}

type TxResponse struct {
	Code uint64
}

type Transaction struct {
	Body Body
}

type Body struct {
	Messages []Message
}

type Message struct {
	Type            string `json:"@type"`
	TxId            string
	TxByteCode      string
	ZkOracleAddress string
	MintOrBurn      bool
	BtcValue        string
	QqAccount       string
	EncryptScalar   string
	TwilightAddress string
}

type ResultPubSub struct {
	Blockhash    string
	Blockheight  string
	Transactions []Message
}

type PayloadHttpReq struct {
	Txid string `json:"ID"`
	Tx   string `json:"Tx"`
	Fee  uint64 `json:"Fee"`
}

type PayloadBurnReq struct {
	BtcValue        uint64 `json:"btc_value"`
	QqAccount       string `json:"qq_account"`
	EncryptScalar   string `json:"encrypt_scalar"`
	TwilightAddress string `json:"twilight_address"`
}

type PayloadPubsub struct {
	topic string
	data  ResultPubSub
}

type Subscriber struct {
	ch chan PayloadPubsub
}

type PubSub struct {
	mu          sync.RWMutex
	subscribers map[string][]*Subscriber
}

type BroadcastRefundMsg struct {
	ReserveId      string `json:"reserveId"`
	RoundId        string `json:"roundId"`
	SignedRefundTx string `json:"signedRefundTx"`
	JudgeAddress   string `json:"judgeAddress"`
}

type BroadcastRefundMsgResp struct {
	BroadcastRefundMsg []BroadcastRefundMsg
}

type BroadcastSweepMsg struct {
	ReserveId     string `json:"reserveId"`
	RoundId       string `json:"roundId"`
	SignedSweepTx string `json:"signedRefundTx"`
	JudgeAddress  string `json:"judgeAddress"`
}

type BroadcastSweepMsgResp struct {
	BroadcastSweepMsg []BroadcastSweepMsg
}
