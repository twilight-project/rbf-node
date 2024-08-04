package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"github.com/spf13/viper"
	"github.com/twilight-project/rbf-node/eventhandler"
	"github.com/twilight-project/rbf-node/types"
	"github.com/twilight-project/rbf-node/utils"
)

var DbConn *sql.DB

func initialize() {
	viper.AddConfigPath("./configs")
	viper.SetConfigName("config") // Register config file name (no extension)
	viper.SetConfigType("json")   // Look for specific type
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Println("Error reading config file: ", err)
	}

	walletName := viper.GetString("btc_core_wallet_name")

	if walletName == "" {
		fmt.Println("Error: wallet_name is required")
		os.Exit(1)
	}

	// if newWallet == true {
	// 	_, err := utils.CreateWallet(walletName)
	// 	if err != nil {
	// 		fmt.Println("Failed to create wallet: ", err)
	// 		os.Exit(1)
	// 	}
	// }
	//DbConn = db.InitDB()
}

func main() {
	
	// Write go code to find the hash256(hash256) of a byte string

	initialize()
	eventhandler.BroadcastSweep(DbConn)
	go eventhandler.NyksEventListener("broadcast_tx_sweep", "broadcastSweep", DbConn)
	go utils.BroadcastOnBtc(DbConn)
	go http.HandleFunc("/rbf", handleRequest)
	fmt.Println(http.ListenAndServe(":8080", nil))
	go utils.ConfirmTx(DbConn)
	utils.CheckPinning(DbConn)
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	// Read the request body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	// Parse the request body as JSON
	var req types.RBFRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Error parsing request body", http.StatusBadRequest)
		return
	}

	wireTransaction, err := utils.CreateTxFromHex(req.Txhex)
	// TODO: Add your logic here
	utils.ReplaceByFee(wireTransaction, req.Amount)
}
