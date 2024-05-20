# RBF Node

This Node pays the fee to run sweep tx on btc chain and has tx pinning monitor in it which notifies if there is a tx pinning attempt.

This Node performs the following tasks
1. It subscibes to the eventhandler on nyks chain via websocket retrieves new sweep Txs from the nyks chain, adds new inputs to cover the fee for the tx and broadcasts it on BTC chain.
2. Keeps track of raw mempool transactions to see if tx pinning was attemped and notifies if there is an attempt of tx pinning.
3. It has a local server running which takes tx and amount, creates a RBF tx with new inputs added to increase the fee by the provided amount.


## Setup
To run the RBF node please ensure the following pre reqs are completed.

1. RBF node uses BTC core node / wallet to keep track of utxos and signing purposes respectively. so a bitcoin core wallet under users control and a node to which user can connect to is needed. please refer [here](https://bitcoin.org/en/full-node) on how t0 do this. 
2. It also uses postgres sql to store the transaction until it is accepted. postgres needs to be setup and the schema provided below must be applied. please refer [here.](https://www.digitalocean.com/community/tutorials/how-to-install-postgresql-on-ubuntu-20-04-quickstart)
3. A Nyks chain Node with api's enabled.

### Configurations
Below is the sample for config file placed in configs folder. once you clone the repo, please ensure that the values are updated according to your environment

```json
{
    "nyksd_url": "https://nyks.twilight-explorer.com/api",
    "nyksd_socket_url" : "ws://147.182.235.183:26657/websocket",
    "btc_node_ip_and_port": "143.244.138.170:8332",
    "btc_node_username": "bitcoin",
    "btc_node_password": "P1",
    "btc_core_wallet_name": "rbfwallet",
    "DB_host": "MyDB",
    "DB_port": "5432",
    "DB_user": "root",
    "DB_password": "P1",
    "DB_name": "rbf"
 }
 ```

 ### Build and run
 once the configurations are set and the schema is applied run the below commands.
 ```shell
 go build .
 ```

 ```shell
 go run .
 ```

### DB Schema
Use the below sql command to create the required table in postgres

```sql
CREATE TABLE signed_tx (
    tx bytea NOT NULL,
    unlock_height bigint NOT NULL
);
```

### RBF
Once the system is running it will automatically add fees to the sweep tx and will keep an eye out for tx pinning. user can manually initaite a request to increase the fee. A sample request to initiate an increase in fee via rbf tx is as below

```shell
curl -X POST -H "Content-Type: application/json" -d '{"txhex":"abc123","amount":10}' http://localhost:8080/rbf/
```