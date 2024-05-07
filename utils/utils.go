package utils

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/spf13/viper"
	"github.com/twilight-project/rbf-node/db"
	"github.com/twilight-project/rbf-node/types"
)

func BtcToSats(btc float64) int64 {
	return int64(btc * 1e8)
}

func GetFeeFromBtcNode(tx *wire.MsgTx) (int64, error) {
	client := getBitcoinRpcClient()
	result, err := client.EstimateSmartFee(2, &btcjson.EstimateModeConservative)
	if err != nil {
		fmt.Println("Failed to get fee from btc node : ", err)
		log.Fatal(err)
	}
	fmt.Printf("Estimated fee per kilobyte for a transaction to be confirmed within 2 blocks: %f BTC\n", *result.FeeRate)
	feeRate := BtcToSats(*result.FeeRate)
	fmt.Printf("Estimated fee per kilobyte for a transaction to be confirmed within 2 blocks: %d Sats\n", feeRate)
	baseSize := tx.SerializeSizeStripped()
	totalSize := tx.SerializeSize()
	weight := (baseSize * 3) + totalSize
	vsize := (weight + 3) / 4
	fmt.Println("tx size in bytes : ", vsize)
	fee := float64(vsize) * float64(feeRate/1024)
	return int64(fee), nil
}

func getBitcoinRpcClient() *rpcclient.Client {
	host := viper.GetString("btc_node_ip_and_port")
	walletName := viper.GetString("wallet_name")

	host = host + "/wallet/" + walletName
	connCfg := &rpcclient.ConnConfig{
		Host:         host,
		User:         fmt.Sprintf("%v", viper.Get("btc_node_username")),
		Pass:         fmt.Sprintf("%v", viper.Get("btc_node_password")),
		HTTPPostMode: true,
		DisableTLS:   true,
	}

	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		fmt.Println("Failed to connect to the Bitcoin client : ", err)
	}

	return client
}

func BroadcastBtcTransaction(tx *wire.MsgTx) {
	client := getBitcoinRpcClient()
	txHash, err := client.SendRawTransaction(tx, true)
	if err != nil {
		fmt.Println("Failed to broadcast transaction : ", err)
	}

	defer client.Shutdown()
	fmt.Println("broadcasted btc transaction, txhash : ", txHash)
}

func CreateTxFromHex(txHex string) (*wire.MsgTx, error) {
	// Decode the transaction hex string
	txBytes, err := hex.DecodeString(txHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex string: %v", err)
	}

	// Create a new transaction object
	tx := wire.NewMsgTx(wire.TxVersion)

	// Deserialize the transaction bytes
	err = tx.Deserialize(bytes.NewReader(txBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize transaction: %v", err)
	}

	return tx, nil
}

// func CreateWallet(walletName string) (*btcjson.CreateWalletResult, error) {
// 	var err error
// 	connCfg := &rpcclient.ConnConfig{
// 		Host:         fmt.Sprintf("%v", viper.Get("btc_node_ip_and_port")),
// 		User:         fmt.Sprintf("%v", viper.Get("btc_node_username")),
// 		Pass:         fmt.Sprintf("%v", viper.Get("btc_node_password")),
// 		HTTPPostMode: true,
// 		DisableTLS:   true,
// 	}

// 	client, err := rpcclient.New(connCfg, nil)
// 	if err != nil {
// 		fmt.Println("Failed to connect to the Bitcoin client : ", err)
// 	}

// 	defer client.Shutdown()

// 	var walletsRaw json.RawMessage
// 	walletsRaw, err = client.RawRequest("listwallets", nil)
// 	if err != nil {
// 		fmt.Println("Error listing wallets: ", err)
// 		return nil, err
// 	}

// 	var wallets []string
// 	err = json.Unmarshal(walletsRaw, &wallets)
// 	if err != nil {
// 		fmt.Println("Error unmarshalling wallets: ", err)
// 		return nil, err
// 	}

// 	for _, wallet := range wallets {
// 		if wallet == walletName {
// 			fmt.Println("Wallet already exists")
// 			return nil, err
// 		}
// 	}

// 	var wallet *btcjson.CreateWalletResult
// 	wallet, err = client.CreateWallet(walletName, rpcclient.WithCreateWalletAvoidReuse(), rpcclient.WithCreateWalletBlank())
// 	if err != nil {
// 		fmt.Println("Failed to create wallet: ", err)
// 		return nil, err
// 	}

// 	fmt.Println("Wallet created successfully")
// 	return wallet, nil
// }

func GetUnspentUTXOs(walletName string) ([]btcjson.ListUnspentResult, error) {
	client := getBitcoinRpcClient()
	defer client.Shutdown()

	// Load the wallet
	_, err := client.LoadWallet(walletName)
	if err != nil {
		fmt.Println("Failed to load wallet: ", err)
		return nil, err
	}

	// Get the unspent transaction outputs
	utxos, err := client.ListUnspent()
	if err != nil {
		fmt.Println("Failed to get unspent UTXOs: ", err)
		return nil, err
	}

	return utxos, nil
}

func AddInputsToCoverFee(tx *wire.MsgTx, walletName string, fee int64) (*wire.MsgTx, int64, error) {
	client := getBitcoinRpcClient()
	defer client.Shutdown()

	// Get the total value of the existing inputs
	totalInputValue := int64(0)
	for _, txIn := range tx.TxIn {
		utxo, err := client.GetTxOut(&txIn.PreviousOutPoint.Hash, txIn.PreviousOutPoint.Index, true)
		if err != nil {
			return nil, 0, err
		}
		totalInputValue += int64(utxo.Value)
	}

	totalOutputValue := int64(0)
	for _, txOut := range tx.TxOut {
		totalOutputValue += int64(txOut.Value)
	}

	feeInputs := int64(0)
	// If the total input value is less than the estimated fee, add new inputs to the transaction
	if totalInputValue-totalOutputValue < fee {
		utxos, err := GetUnspentUTXOs(walletName)
		if err != nil {
			return nil, 0, err
		}

		for _, utxo := range utxos {
			if totalInputValue-totalOutputValue >= fee {
				if totalInputValue-totalOutputValue > fee {
					addr, err := client.GetNewAddress(walletName)
					if err != nil {
						fmt.Println("Error getting new address: ", err)
						return nil, 0, err
					}

					// Generate the pay-to-address script.
					destinationAddrByte, err := txscript.PayToAddrScript(addr)
					if err != nil {
						fmt.Println("Error generating pay-to-address script:", err)
						return nil, 0, err
					}
					amount := totalInputValue - totalOutputValue - fee
					txOut := wire.NewTxOut(amount, destinationAddrByte)
					tx.AddTxOut(txOut)
				}
				break
			}

			hash, err := chainhash.NewHashFromStr(utxo.TxID)
			if err != nil {
				return nil, 0, err
			}
			outPoint := wire.NewOutPoint(hash, utxo.Vout)
			txIn := wire.NewTxIn(outPoint, nil, nil)
			tx.AddTxIn(txIn)

			totalInputValue += int64(utxo.Amount)
			feeInputs += 1
		}
	}
	return tx, feeInputs, nil
}

func SignNewFeeInputs(tx *wire.MsgTx, n int64) (*wire.MsgTx, error) {
	client := getBitcoinRpcClient()

	inputs := tx.TxIn[int64(len(tx.TxIn))-n:]
	witnessInputs := make([]btcjson.RawTxWitnessInput, len(inputs))
	for i, input := range inputs {
		witnessInputs[i] = btcjson.RawTxWitnessInput{
			Txid: input.PreviousOutPoint.Hash.String(),
			Vout: input.PreviousOutPoint.Index,
		}
	}

	// Sign the new transaction

	signedTx, _, err := client.SignRawTransactionWithWallet3(tx, witnessInputs, rpcclient.SigHashType(txscript.SigHashAll|txscript.SigHashAnyOneCanPay))
	if err != nil {
		fmt.Println("Failed to sign transaction: ", err)
		return nil, err
	}
	return signedTx, nil

}

func txToHex(tx *wire.MsgTx) (string, error) {
	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(buf.Bytes()), nil
}

func GetBroadCastedRefundTx() types.BroadcastRefundMsg {
	nyksd_url := fmt.Sprintf("%v", viper.Get("nyksd_url"))
	path := fmt.Sprintf("/twilight-project/nyks/bridge/broadcast_tx_refund_all")
	resp, err := http.Get(nyksd_url + path)
	if err != nil {
		fmt.Println("error getting broadcasted refund : ", err)
	}
	//We Read the response body on the line below.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("error getting broadcasted refund body : ", err)
	}

	a := types.BroadcastRefundMsgResp{}
	err = json.Unmarshal(body, &a)
	if err != nil {
		fmt.Println("error unmarshalling broadcasted refund : ", err)
	}
	return a.BroadcastRefundMsg[0]
}

func GetBroadCastedSweepTx() types.BroadcastSweepMsg {
	nyksd_url := fmt.Sprintf("%v", viper.Get("nyksd_url"))
	path := fmt.Sprintf("/twilight-project/nyks/bridge/broadcast_tx_sweep_all")
	resp, err := http.Get(nyksd_url + path)
	if err != nil {
		fmt.Println("error getting broadcasted refund : ", err)
	}
	//We Read the response body on the line below.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("error getting broadcasted refund body : ", err)
	}

	a := types.BroadcastSweepMsgResp{}
	err = json.Unmarshal(body, &a)
	if err != nil {
		fmt.Println("error unmarshalling broadcasted refund : ", err)
	}
	return a.BroadcastSweepMsg[0]
}

func BroadcastOnBtc(dbconn *sql.DB) {
	fmt.Println("Started Btc Broadcaster")
	client := getBitcoinRpcClient()
	for {
		blockHeight, err := client.GetBlockCount()
		if err != nil {
			log.Fatalf("Error getting block count: %v", err)
		}
		txs := db.QuerySignedTx(dbconn, blockHeight)
		for _, tx := range txs {
			transaction := hex.EncodeToString(tx)
			wireTransaction, err := CreateTxFromHex(transaction)
			if err != nil {
				fmt.Println("error decodeing signed transaction btc broadcaster : ", err)
			}
			BroadcastBtcTransaction(wireTransaction)
			db.DeleteSignedTx(dbconn, tx)
		}
	}
}

func DecodeBtcScript(script string) string {
	decoded, err := hex.DecodeString(script)
	if err != nil {
		fmt.Println("Error decoding script Hex : ", err)
	}
	decodedScript, err := txscript.DisasmString(decoded)
	if err != nil {
		fmt.Println("Error decoding script : ", err)
	}

	return decodedScript
}

func GetHeightFromScript(script string) int64 {
	// Split the decoded script into parts
	height := int64(0)
	parts := strings.Split(script, " ")
	if len(parts) == 0 {
		return height
	}
	// Reverse the byte order
	for i, j := 0, len(parts[0])-2; i < j; i, j = i+2, j-2 {
		parts[0] = parts[0][:i] + parts[0][j:j+2] + parts[0][i+2:j] + parts[0][i:i+2] + parts[0][j+2:]
	}
	// Convert the first part from hex to decimal
	height, err := strconv.ParseInt(parts[0], 16, 64)
	if err != nil {
		fmt.Println("Error converting block height from hex to decimal:", err)
	}

	return height
}
