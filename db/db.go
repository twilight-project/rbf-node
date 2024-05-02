package db

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/spf13/viper"
)

func InitDB() *sql.DB {
	psqlconn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", viper.Get("DB_host"), viper.Get("DB_port"), viper.Get("DB_user"), viper.Get("DB_password"), viper.Get("DB_name"))
	db, err := sql.Open("postgres", psqlconn)
	if err != nil {
		log.Println("DB error : ", err)
		panic(err)
	}
	fmt.Println("DB initialized")
	return db
}

func QuerySignedTx(dbconn *sql.DB, unlock_height int64) [][]byte {
	DB_reader, err := dbconn.Query("select tx from signed_tx where unlock_height <= $1", unlock_height)
	if err != nil {
		fmt.Println("An error occured while query script signed tx: ", err)
	}

	defer DB_reader.Close()
	txs := [][]byte{}

	for DB_reader.Next() {
		tx := []byte{}
		err := DB_reader.Scan(
			&tx,
		)
		if err != nil {
			fmt.Println(err)
		}

		txs = append(txs, tx)
	}
	return txs
}

func DeleteSignedTx(dbconn *sql.DB, tx []byte) {
	_, err := dbconn.Exec("DELETE FROM signed_tx WHERE tx = $1", tx)
	if err != nil {
		fmt.Println("An error occurred while executing delete signed tx: ", err)
	} else {
		fmt.Println("Transaction successfully deleted")
	}
}

func InsertSignedtx(dbconn *sql.DB, tx []byte, unlock_height int64) {
	_, err := dbconn.Exec("INSERT into signed_tx VALUES ($1, $2)",
		tx,
		unlock_height,
	)
	if err != nil {
		fmt.Println("An error occured while executing insert signed sweep tx: ", err)
	}
}
