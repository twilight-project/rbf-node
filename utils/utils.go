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
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire" // Add this import statement
	"github.com/spf13/viper"
	"github.com/twilight-project/rbf-node/db"
	"github.com/twilight-project/rbf-node/types"
)

func BtcToSats(btc float64) int64 {
	return int64(btc * 1e8)
}

func GetFeeFromBtcNode(tx *wire.MsgTx) (int64, int, error) {
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
	//fee := float64(vsize) * float64(feeRate/1024)
	return feeRate, vsize, nil
}

func getBitcoinRpcClient() *rpcclient.Client {
	host := viper.GetString("btc_node_ip_and_port")
	walletName := viper.GetString("btc_core_wallet_name")

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

func DecodeTransaction(txStr string)(*btcjson.TxRawResult){
	client := getBitcoinRpcClient()
	// Decode the transaction hex string
	txBytes, _ := hex.DecodeString(txStr)
	rawTx, err:= client.DecodeRawTransaction(txBytes)
	if err != nil {
		fmt.Println("Failed to decode transaction : ", err)
	}
	return rawTx
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
	//_, err := client.LoadWallet(walletName)
	//if err != nil {
	//	fmt.Println("Failed to load wallet: ", err)
	//	return nil, err
	//}

	// Get the unspent transaction outputs
	utxos, err := client.ListUnspent()
	if err != nil {
		fmt.Println("Failed to get unspent UTXOs: ", err)
		return nil, err
	}
	

	return utxos, nil
}
func convertPsbtStringToPacket(psbtString string) (*wire.MsgTx, error) {
	reader := strings.NewReader(psbtString)
    // Convert the byte slice to a psbt.Packet
    packet, err := psbt.NewFromRawBytes(reader, true)
    if err != nil {
        return nil, fmt.Errorf("failed to convert to psbt.Packet: %v", err)
    }
	tx, err:=psbt.Extract(packet)
	if err != nil {
		return nil, fmt.Errorf("failed to extract transaction from psbt: %v", err)
	}
    return tx, nil
}
func AddTestFeeUtxos(tx *wire.MsgTx, walletName string, fee int64){
	client := getBitcoinRpcClient()
	defer client.Shutdown()
	// create a new transaction that will be used to cover the fee
	// create an output with the amount of the fee
	// let the wallet choose apropriate inputs to cover the fee
	newFeeAddress, err := client.GetNewAddress(walletName)
	if err != nil {
		fmt.Println("Error getting new address for fee tx: ", err)
		return
	}
	amount1 := btcutil.Amount(fee)
	// create new PSBTOutput
	output := btcjson.NewPsbtOutput(newFeeAddress.EncodeAddress(),amount1)
	// Wrap the output in a slice
    outputs := []btcjson.PsbtOutput{output}
	changePosition := int64(1)
	opts := &btcjson.WalletCreateFundedPsbtOpts{
		ChangePosition: &changePosition,
	}
	// create a new transaction using the fee address
	fundedResult, err := client.WalletCreateFundedPsbt([]btcjson.PsbtInput{}, outputs, nil, opts, nil)
	if err != nil {
		fmt.Println("Failed to create PSBT fee transaction: ", err)
		return
	}
	// convert the psbt to hex
	psbt := fundedResult.Psbt
	isSign:= true
	// sign the psbt 
	signedPsbt, err := client.WalletProcessPsbt(psbt, &isSign, rpcclient.SigHashAll, nil)
	if err != nil {
		fmt.Println("Failed to sign PSBT: ", err)
		return
	}
	fmt.Println("PSBT: ", signedPsbt.Psbt)
	fmt.Println("Complete: ",signedPsbt.Complete)
	// convert psbt to psbt.Packet
	feeTx, err := convertPsbtStringToPacket(signedPsbt.Psbt)
	if err != nil {
		fmt.Println("Failed to convert PSBT string to packet: ", err)
		return
	}
	// convert to hex
	feeTxHex, _ := ConvertTxtoHex(feeTx)
	// convert the psbt to hex
	fmt.Println("Fee tx: ", feeTxHex)
	// broadcast the fee transaction 
	txHash, err := client.SendRawTransaction(feeTx, true)
	if err != nil {
		fmt.Println("Failed to broadcast fee transaction : ", err)
		return
	}

	fmt.Println("broadcasted btc fee transaction, txhash : ", txHash)

	outPoint := wire.NewOutPoint(txHash, 0)
	txIn := wire.NewTxIn(outPoint, nil, nil)
	tx.AddTxIn(txIn)
		
}
func CombineRawTransactions(sweepTx *wire.MsgTx, feeTx *wire.MsgTx) *wire.MsgTx {
	txIn := feeTx.TxIn[1] 
	sweepTx.AddTxIn(txIn)
	//remove the index 1 from sweeptx
	sweepTx.TxIn = append(sweepTx.TxIn[:1], sweepTx.TxIn[1+1:]...)
	return sweepTx
}
func ConvertTxtoHex(tx *wire.MsgTx) (string, error) {
	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(buf.Bytes()), nil
}
func AddInputsToCoverFee(tx *wire.MsgTx, walletName string, fee int64) (*wire.MsgTx, int64, []btcjson.RawTxWitnessInput, error) {
	client := getBitcoinRpcClient()
	defer client.Shutdown()

	// Get the total value of the existing inputs
	totalInputValue := int64(0)
	for _, txIn := range tx.TxIn {
		utxo, err := client.GetTxOut(&txIn.PreviousOutPoint.Hash, txIn.PreviousOutPoint.Index, true)
		if err != nil {
			return nil, 0, nil, err
		}
		totalInputValue += int64(utxo.Value)
	}

	totalOutputValue := int64(0)
	for _, txOut := range tx.TxOut {
		totalOutputValue += int64(txOut.Value)
	}
	witnessInputs := make([]btcjson.RawTxWitnessInput,0)
	feeInputs := int64(0)
	// If the total input value is less than the estimated fee, add new inputs to the transaction
	if totalInputValue-totalOutputValue < fee {
		fmt.Println("Total input value is less than the estimated fee")
		utxos, err := GetUnspentUTXOs(walletName)
		if err != nil {
			return nil, 0, nil, err
		}
		// create witness utxo inputs here
		// as the information is easily available here
		fmt.Println("utxos: ", utxos)
		for _, utxo := range utxos {
			if totalInputValue-totalOutputValue >= fee {
				if totalInputValue-totalOutputValue > fee {
					addr, err := client.GetNewAddress(walletName)
					if err != nil {
						fmt.Println("Error getting new address: ", err)
						return nil, 0, nil, err
					}
					fmt.Println("new address", addr)

					// Generate the pay-to-address script.
					destinationAddrByte, err := txscript.PayToAddrScript(addr)
					if err != nil {
						fmt.Println("Error generating pay-to-address script:", err)
						return nil, 0, nil, err
					}
					amount := totalInputValue - totalOutputValue - fee
					fmt.Println("amount: ", amount)
					txOut := wire.NewTxOut(amount, destinationAddrByte)
					tx.AddTxOut(txOut)
				}
				break
			}

			hash, err := chainhash.NewHashFromStr(utxo.TxID)
			if err != nil {
				return nil, 0, nil, err
			}
			outPoint := wire.NewOutPoint(hash, utxo.Vout)
			txIn := wire.NewTxIn(outPoint, nil, nil)
			tx.AddTxIn(txIn)
			// add witness information here
			amountPtr:= &utxo.Amount
			witness:= btcjson.RawTxWitnessInput{
				Txid: utxo.TxID,
				Vout: utxo.Vout,
				ScriptPubKey: utxo.ScriptPubKey,
				Amount: amountPtr,

			}
			witnessInputs = append(witnessInputs, witness)
			totalInputValue += int64(utxo.Amount)
			feeInputs += 1
		}
	}
	return tx, feeInputs, witnessInputs, nil
}

func SignNewFeeInputs(tx *wire.MsgTx, n int64, witness []btcjson.RawTxWitnessInput) (*wire.MsgTx, error) {
	client := getBitcoinRpcClient()

	// Sign the new transaction

	signedTx, complete, err := client.SignRawTransactionWithWallet3(tx, nil, "ALL")
	fmt.Println("complete: ", complete)
	if err != nil {
		fmt.Println("Failed to sign transaction: ", err)
		return nil, err
	}
	return signedTx, nil

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

func GetBroadCastedSweepTx() (types.BroadcastTxSweepMsg, error) {
	nyksd_url := fmt.Sprintf("%v", viper.Get("nyksd_url"))
	path := fmt.Sprintf("/twilight-project/nyks/bridge/broadcast_tx_sweep_all")
	resp, err := http.Get(nyksd_url + path)
	if err != nil {
		fmt.Println("error getting broadcasted sweep : ", err)
	}
	//We Read the response body on the line below.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("error getting broadcasted sweep body : ", err)
	}

	a := types.BroadcastSweepMsgResp{}
	err = json.Unmarshal(body, &a)
	if err != nil {
		fmt.Println("error unmarshalling broadcasted sweep : ", err)
	}
	if len(a.BroadcastTxSweepMsg) > 0 {
		return a.BroadcastTxSweepMsg[0], nil
	}
	return types.BroadcastTxSweepMsg{}, fmt.Errorf("No sweep transaction found")
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
	part := 25
	parts := strings.Split(script, " ")
	if len(parts) == 0 {
		return height
	}
	// Reverse the byte order
	for i, j := 0, len(parts[part])-2; i < j; i, j = i+2, j-2 {
		parts[part] = parts[part][:i] + parts[part][j:j+2] + parts[part][i+2:j] + parts[part][i:i+2] + parts[part][j+2:]
	}
	// Convert the first part from hex to decimal
	height, err := strconv.ParseInt(parts[part], 16, 64)
	if err != nil {
		fmt.Println("Error converting block height from hex to decimal:", err)
	}

	return height
}
func CreateTxForFeeUtxo(){
	//client:= getBitcoinRpcClient()
	// create a new transaction
	tx := wire.NewMsgTx(wire.TxVersion)
	// add the inputs
	// add the outputs
	// add the witness information
	// sign the transaction
	// broadcast the transaction
	//wireTransaction,_:= (tx)
		BroadcastBtcTransaction(tx)
}
func CheckPinning(dbconn *sql.DB) {
	client := getBitcoinRpcClient()
	defer client.Shutdown()

	for {
		// Get the list of transactions in the mempool
		txs := db.QuerySignedTxAll(dbconn)
		for _, tx := range txs {
			transaction := hex.EncodeToString(tx)
			wireTransaction, err := CreateTxFromHex(transaction)
			if err != nil {
				fmt.Println("error decodeing signed transaction btc broadcaster : ", err)
				continue
			}
			utxo := wireTransaction.TxHash().String()

			txids, err := client.GetRawMempool()
			if err != nil {
				fmt.Println("Failed to get mempool transactions: ", err)
				continue
			}

			// Check each transaction in the mempool
			for _, txid := range txids {
				rawTx, err := client.GetRawTransaction(txid)
				if err != nil {
					fmt.Println("Failed to get raw transaction: ", err)
					continue
				}

				decodedTx := rawTx.MsgTx()
				if err != nil {
					log.Fatal(err)
				}

				// Check each input in the transaction
				for _, vin := range decodedTx.TxIn {
					if vin.PreviousOutPoint.Hash.String() == utxo {
						fmt.Printf("Transaction %s in the mempool spends UTXO \n", txid)
					}
				}
			}
		}
	}
}

func ReplaceByFee(tx *wire.MsgTx, amount int32) {
	walletName := viper.GetString("btc_core_wallet_name")
	client := getBitcoinRpcClient()
	defer client.Shutdown()

	newAmountAdded := float64(0)

	utxos, err := GetUnspentUTXOs(walletName)
	if err != nil {
		fmt.Println("Failed to get unspent UTXOs: ", err)
	}
	for _, utxo := range utxos {
		for _, vin := range tx.TxIn {
			if utxo.TxID == vin.PreviousOutPoint.Hash.String() {
				continue
			}
		}

		hash, err := chainhash.NewHashFromStr(utxo.TxID)
		if err != nil {
			fmt.Println("Failed to create hash from string: ", err)
			return
		}
		outPoint := wire.NewOutPoint(hash, utxo.Vout)
		txIn := wire.NewTxIn(outPoint, nil, nil)
		tx.AddTxIn(txIn)

		input := tx.TxIn[int64(len(tx.TxIn))]
		witnessInputs := make([]btcjson.RawTxWitnessInput, 1)
		witnessInputs[1] = btcjson.RawTxWitnessInput{
			Txid: input.PreviousOutPoint.Hash.String(),
			Vout: input.PreviousOutPoint.Index,
		}

		tx, _, err := client.SignRawTransactionWithWallet3(tx, witnessInputs, rpcclient.SigHashType(txscript.SigHashAll|txscript.SigHashAnyOneCanPay))

		newAmountAdded += utxo.Amount

		if newAmountAdded == float64(amount) {
			break
		}

		if newAmountAdded > float64(amount) {
			addr, err := client.GetNewAddress(walletName)
			if err != nil {
				fmt.Println("Error getting new address: ", err)
				return
			}

			// Generate the pay-to-address script.
			destinationAddrByte, err := txscript.PayToAddrScript(addr)
			if err != nil {
				fmt.Println("Error generating pay-to-address script:", err)
				return
			}
			amount := newAmountAdded - float64(amount)
			txOut := wire.NewTxOut(int64(amount), destinationAddrByte)
			tx.AddTxOut(txOut)
			break
		}

	}

	if newAmountAdded < float64(amount) {
		fmt.Println("Insufficient funds")
		return
	}

	BroadcastBtcTransaction(tx)
	fmt.Printf("Broadcasted RBF transaction with txid %s\n", tx.TxHash().String())
}

func ConfirmTx(dbconn *sql.DB) {
	client := getBitcoinRpcClient()
	defer client.Shutdown()

	for {
		signed_txs := db.QuerySignedTxAll(dbconn)

		for _, tx := range signed_txs {
			transaction := hex.EncodeToString(tx)
			wireTransaction, err := CreateTxFromHex(transaction)
			txHash, err := chainhash.NewHashFromStr(wireTransaction.TxHash().String())
			if err != nil {
				fmt.Println(err)
				continue
			}

			txHashResult, err := client.GetRawTransactionVerbose(txHash)
			if err != nil {
				fmt.Println(err)
				continue
			}

			fmt.Printf("Transaction details: %+v\n", txHashResult)
			db.DeleteSignedTx(dbconn, tx)
		}
	}
}


// Initialize the map
    // amounts := make(map[btcutil.Address]btcutil.Amount)
	// amount1 := btcutil.Amount(fee) // takes value in Sats
	// amounts[newFeeAddress] = amount1
	// feeTx, err:= client.CreateRawTransaction(nil,amounts,nil)
	// if err != nil {
	// 	fmt.Println("Failed to create fee transaction: ", err)
	// 	return
	// }
	// feeTxHex,_ := ConvertTxtoHex(feeTx)
	// fmt.Println("Fee tx: ", feeTxHex)
	// // fund the new transaction

	// // Create a default FundRawTransactionOpts
    // opts := btcjson.FundRawTransactionOpts{
    //     ChangeAddress: nil, // or provide a valid address
    //     ChangePosition: nil,
    //     IncludeWatching: nil,
    //     LockUnspents: nil,
    //     FeeRate: nil,
    //     SubtractFeeFromOutputs: nil,
    //     Replaceable: nil,
    //     ConfTarget: nil,
    //     EstimateMode: nil,
    // }
	// isWitness := true
	// fundedFeeTxResult, err := client.FundRawTransaction(feeTx, opts, &isWitness)
	// if err != nil {
	// 	fmt.Println("Failed to fund fee transaction: ", err)
	// 	return
	// }
	// feeTxFunded := fundedFeeTxResult.Transaction
	// // sign the tx 
	// signedFeeTx, complete, err := client.SignRawTransactionWithWallet(feeTxFunded)
	// if err != nil {
	// 	fmt.Println("Failed to sign fee utxo creating transaction: ", err)
	// 	return
	// }
	// if !complete {
	// 	fmt.Println("Fee creation Transaction not complete")
	// 	return
	// }
	// convert the tx to hex
	//signedFeeTxHex, err := ConvertTxtoHex(signedFeeTx)
	//fmt.Println("Signed Fee tx: ", signedFeeTxHex)