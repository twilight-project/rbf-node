package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/spf13/viper"
	"github.com/twilight-project/rbf-node/db"
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
	DbConn = db.InitDB()
}

func main() {
	initialize()
	go eventhandler.NyksEventListener("broadcast_tx_sweep", "broadcastSweep", DbConn)
	go utils.BroadcastOnBtc(DbConn)
	go http.HandleFunc("/rbf", handleRequest)
	fmt.Println(http.ListenAndServe(":8080", nil))
	go utils.ConfirmTx(DbConn)
	utils.CheckPinning(DbConn)

	// x := "01000000000101e71708c349cb23c333bfe83f673a09eec9f1ac0c88e315f9c1eb55ad81ed7ef5000000000085d00c0001a4900000000000002200204593ced53eddb4d6695bc34d97fe1fbc9ecade6564e1bd5b2cfca7b4cb31fe3e0720bbd32040d3fa8fd784d3b784d206443b1a644b6062680ed576298aabefc329c500483045022100c71b82a058262795aeecb6d309f2278d3d437562485598558109d2070fff322202206bcdb3a17510973f9c1ce835cfce0ebb7d328819a770f7b6b9f91b5d0cf276a10147304402200af72303f8357759d6e27715c1a4ddc5da57f51708346e5fc14766e796e8aa550220386102b032f28b582af686e2eb1b37035b6e23826ebd8b16646b3302e774314501483045022100b562ce717950901dde292118ca2b5b30ded0288091d330d6cec86b60321ed2a6022017a6f0e37f20a005f67155301bb6a1ede8e87a1212fbb6f0ba77635bc2bff374014830450221009f196565edd3f976e3b47578d9132a7ba24179d3e644192f82cabf937a1d614502200cf7346e82d0091c8b3ac157346fc9d3413db3295d3e606211d93a873f343a4701fd1e010389d00cb175542103b03fe3da02ac2d43a1c2ebcfc7b0497e89cc9f62b513c0fc14f10d3d1a2cd5e62102ca505bf28698f0b6c26114a725f757b88d65537dd52a5b6455a9cac9581f10552103bb3694e798f018a157f9e6dfb51b91f70a275443504393040892b52e45b255c32103e2f80f2f5eb646df3e0642ae137bf13f5a9a6af4c05688e147c64e8fae196fe121038b38721dbb1427fd9c65654f87cb424517df717ee2fea8b0a5c376a17349416721033e72f302ba2133eddd0c7416943d4fed4e7c60db32e6b8c58895d3b26e24f92756af82012088a914dbefa70a0e35c33c66e56129552a69baf86ee9e78773642102ca505bf28698f0b6c26114a725f757b88d65537dd52a5b6455a9cac9581f1055ac640394d00cb27568688ad00c00"
	// sweepTx, _ := utils.CreateTxFromHex(x)
	// wintess := hex.EncodeToString(sweepTx.TxIn[0].Witness[len(sweepTx.TxIn[0].Witness)-1])
	// decodedScript := utils.DecodeBtcScript(wintess)
	// height := utils.GetHeightFromScript(decodedScript)
	// fmt.Println("Height: ", height)

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
