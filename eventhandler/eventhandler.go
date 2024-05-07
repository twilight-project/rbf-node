package eventhandler

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
	"github.com/twilight-project/rbf-node/db"
	"github.com/twilight-project/rbf-node/utils"
)

func NyksEventListener(event string, functionCall string, dbconn *sql.DB) {
	headers := make(map[string][]string)
	headers["Content-Type"] = []string{"application/json"}
	nyksd_url := fmt.Sprintf("%v", viper.Get("nyksd_socket_url"))
	conn, _, err := websocket.DefaultDialer.Dial(nyksd_url, headers)
	if err != nil {
		fmt.Println("nyks event listerner dial:", err)
	}
	defer conn.Close()

	// Set up ping/pong connection health check
	pingPeriod := 30 * time.Second
	pongWait := 60 * time.Second
	stopChan := make(chan struct{}) // Create the stop channel

	err = conn.SetReadDeadline(time.Now().Add(pongWait))
	if err != nil {
		fmt.Println("error setting read deadline: ", err)
	}
	conn.SetPongHandler(func(string) error {
		err = conn.SetReadDeadline(time.Now().Add(pongWait))
		if err != nil {
			fmt.Println("error setting read deadline: ", err)
		}
		return nil
	})

	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-stopChan: // Listen to the stop channel
				return
			}
		}
	}()

	payload := `{
        "jsonrpc": "2.0",
        "method": "subscribe",
        "id": 0,
        "params": {
            "query": "tm.event='Tx' AND message.action='%s'"
        }
    }`
	payload = fmt.Sprintf(payload, event)

	err = conn.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		fmt.Println("error in nyks event handler: ", err)
		stopChan <- struct{}{} // Signal goroutine to stop
		return
	}

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("error in nyks event handler: ", err)
			stopChan <- struct{}{} // Signal goroutine to stop
			return
		}

		switch functionCall {
		case "broadcastRefund":
			go broadcastRefund()
		case "broadcastSweep":
			go BroadcastSweep(dbconn)
		default:
			log.Println("Unknown function :", functionCall)
		}
	}
}

func broadcastRefund() {
	fmt.Println("broadcasting refund transaction")
}

func BroadcastSweep(dbconn *sql.DB) {
	fmt.Println("broadcasting sweep transaction")
	tx := utils.GetBroadCastedSweepTx()
	sweepTx, err := utils.CreateTxFromHex(tx.SignedSweepTx)
	if err != nil {
		fmt.Println("Failed to create sweep transaction : ", err)
		return
	}

	decodedScript := utils.DecodeBtcScript(hex.EncodeToString(sweepTx.TxIn[0].Witness[len(sweepTx.TxIn[0].Witness)-1]))
	height := utils.GetHeightFromScript(decodedScript)

	fee, err := utils.GetFeeFromBtcNode(sweepTx)
	if err != nil {
		fmt.Println("Failed to get fee from btc node : ", err)
		return
	}

	newTx, n, err := utils.AddInputsToCoverFee(sweepTx, "", fee)
	if err != nil {
		fmt.Println("Failed to add inputs to cover fee : ", err)
		return
	}
	signedTx, err := utils.SignNewFeeInputs(sweepTx, n)
	if err != nil {
		fmt.Println("Failed to sign new fee inputs : ", err)
		return
	}

	fmt.Println("Fee for sweep transaction : ", fee)
	fmt.Println("sweep transaction new inputs : ", newTx)
	fmt.Println("sweep transaction signed inputs : ", signedTx)

	var buf bytes.Buffer
	err = signedTx.Serialize(&buf)
	if err != nil {
		log.Fatalf("Failed to serialize transaction: %v", err)
	}
	byteArray := buf.Bytes()

	db.InsertSignedtx(dbconn, byteArray, height)
}
