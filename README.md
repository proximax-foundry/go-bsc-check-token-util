# go-bsc-check-token-util

The check token util script is a tool to detect the balance of assets in BscScan accounts. Alert will be sent via Telegram if asset balance is found to be less than the threshold balance.

# Getting Started

## Prerequisites

* [Golang](https://golang.org/
) is required (tested on go1.21.5)
* [Telegram Bots API](https://core.telegram.org/bots
) Key and Chat Id

## Clone the Project
```

git clone git@github.com:proximax-foundry/go-bsc-check-token-util.git
cd go-bsc-check-token-util

```

# Configurations
Configurations can be made to the script by changing the values to the fields in config.json.
```json
{
    "botApiKey": "1111111111:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
    "chatID": 111111111,
    "sleep": 60,
    "alarmInterval": 1,
    "walletAddress": [
        "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
    ],
    "tokenList": {
        "bnb": {
            "defaultCurrency": true,
            "tokenContractAddress": "",
            "thresholdBalance": 9999999
        },
        "xpx": {
            "defaultCurrency": false,
            "tokenContractAddress": "0x6F3AAf802F57D045efDD2AC9c06d8879305542aF",
            "thresholdBalance": 9999999999
        }
    }
}
```
* `botApiKey`: Telegram Bot API Key (in String format)
* `chatID`: Telegram Chat ID (in numeric format)
* `sleep`: The time interval (in seconds) of checking the assets' balance
* `alarmInterval`: The time interval (in hours) to send alert to Telegram
* `walletAddress`: BscScan wallet address
* `tokenList`: List of tokens to be checked
* `defaultCurrency`: Set as `false` except BNB
* `tokenContractAddress`: Token contract address
* `thresholdBalance`: The amount where if the asset’s balance falls lower than that, an alert will be sent

Note that the default values in config.json are presented solely as examples.

# Running the Script
```go
go run main.go
```
