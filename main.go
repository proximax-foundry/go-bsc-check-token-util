package main

import (
	"os"
	"fmt"
	"log"
	"math"
	"time"
	"context"
	"reflect"
	"strconv"
	"strings"
	"math/big"
	"encoding/json"
	"github.com/ethereum/go-ethereum/rpc"
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Config struct {
	Sleep         	*int
    ChatID        	*int64
    BotApiKey     	*string
	AlarmInterval	*int
	WalletAddress   []*string
	TokenList       map[string]Token
}

type hexOrDecimalBigInt struct {
	*big.Int
}

type WalletDetail struct {
    Address		string
    Balance		[]string
}

type Token struct {
	DefaultCurrency			*bool // need to check if they are nil
	TokenContractAddress	*string
	ThresholdBalance		*float64
}

var config Config
var client *rpc.Client
var alarmTime time.Time
var quantity *big.Float

func main() {
	err := readConfig()
    if err != nil {
        errHandling(err)
    }

	client, err = rpc.Dial("https://bsc-dataseed.binance.org/")

	if err != nil {
		errHandling(fmt.Errorf("Failed to send RPC call: %v\n", err))
	}
	defer client.Close()

	alarmInterval := time.Duration(*config.AlarmInterval) * time.Hour

    for {
        var targetWallets []WalletDetail
        for _, walletAddress := range config.WalletAddress {
            var targetTokens []string
			for name, detail := range config.TokenList {
				if detail.DefaultCurrency == nil {
					errHandling(fmt.Errorf("Missing value of DefaultCurrency in TokenList in config.json\n"))
				} else if detail.TokenContractAddress == nil {
					errHandling(fmt.Errorf("Missing value of TokenContractAddress in TokenList in config.json\n"))
				} else if detail.ThresholdBalance == nil {
					errHandling(fmt.Errorf("Missing value of ThresholdBalance in TokenList in config.json\n"))
				}
				tokenName := strings.ToUpper(name)
				quantity := big.NewFloat(*detail.ThresholdBalance)

				if *detail.DefaultCurrency {
					bnbBalance, err := getBNBBalance(*walletAddress)
					if err != nil {
						errHandling(fmt.Errorf("Failed to get BNB balance: %v\n", err))
					}
					if bnbBalance.Cmp(quantity) < 0 {
						targetTokens = append(targetTokens, bnbBalance.String() + " " + tokenName)
					}
				} else {
					tokenBalance, err := getTokenBalance(*detail.TokenContractAddress, *walletAddress)
					if err != nil {
						errHandling(fmt.Errorf("Failed to get token balance: %v\n", err))
					}
					if tokenBalance.Cmp(quantity) < 0 {
						targetTokens = append(targetTokens, tokenBalance.String() + " " + tokenName)
					}
				}
			}
            if len(targetTokens) > 0 {
                targetWallets = append(targetWallets, WalletDetail{
                    Address: *walletAddress,
					Balance: targetTokens,
                })
            }
        }
        if len(targetWallets) > 0 && time.Since(alarmTime) > alarmInterval {
            sendAlert(targetWallets)
			alarmTime = time.Now() 
        }
        time.Sleep(time.Duration(*config.Sleep)*time.Second)
    }
}

func checkMissingFields() (error) {
    var missingFields []string

    r := reflect.ValueOf(&config).Elem()
	rt := r.Type()
    for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		rv := reflect.ValueOf(&config)
		value := reflect.Indirect(rv).FieldByName(field.Name)
        if value.IsNil() {
            missingFields = append(missingFields, field.Name)
        }
	}

    if len(missingFields) > 0 {
        errMsg := "Cannot get the value of "
        for j, field := range missingFields {
            errMsg += field
            if j != len(missingFields)-1 {
                errMsg += ", "
            }
        }
        errMsg += " in config.json."
        return fmt.Errorf(errMsg)
    }
    return nil
}

func constructMsg(targets []WalletDetail) (string, []tgbotapi.MessageEntity, error) {
    n := 0
    entities := []tgbotapi.MessageEntity{}

    msg := "The following BSC accounts' tokens require top up:\n\n"
    for i, target := range targets {
        entities = append(entities, tgbotapi.MessageEntity{
            Type: "text_link",
            Offset: 52 + i*43 + n,
            Length: 42,
            URL: "https://bscscan.com/address/" + target.Address,
        })

        msg += target.Address + "\n"
        var balanceMsg string
		for _, balance := range target.Balance {
			balanceMsg = "- " + balance + "\n"
			msg += balanceMsg
			n += len(balanceMsg)
		}
    }
    return msg, entities, nil
}

func errHandling(err error) {
	log.SetOutput(os.Stderr)
    log.Fatal("Error: ", err)
}

func getBNBBalance(address string) (*big.Float, error) {
	var result hexOrDecimalBigInt

	err := client.CallContext(context.Background(), &result, "eth_getBalance", address, "latest")
	if err != nil {
		return nil, err
	}

	balanceWei, success := new(big.Int).SetString(result.String(), 0)
	if !success {
		return nil, fmt.Errorf("Failed to convert balance to big.Int")
	}

	balanceBNB := new(big.Float).Quo(new(big.Float).SetInt(balanceWei), big.NewFloat(1e18))
	return balanceBNB, nil
}

func getTokenBalance(tokenContractAddress string, address string) (*big.Float, error) {
	data := fmt.Sprintf("0x70a08231000000000000000000000000%s", address[2:])

	var result hexOrDecimalBigInt
	err := client.CallContext(context.Background(), &result, "eth_call", map[string]interface{} {
		"to":   tokenContractAddress,
		"input": data,
		}, "latest")
	if err != nil {
		return nil, err
	}

	balance, success := new(big.Int).SetString(result.String(), 0)
	if !success {
		return nil, fmt.Errorf("Failed to convert balance to big.Int")
	}

	decimals, err := getTokenDecimals(tokenContractAddress)
	if err != nil {
		return nil, fmt.Errorf("Failed to get token decimals: %v\n", err)
	}
	balanceFloat := new(big.Float).Quo(new(big.Float).SetInt(balance), big.NewFloat(math.Pow(10, decimals)))
	return balanceFloat, nil
}

func getTokenDecimals(tokenContractAddress string) (float64, error) {
	var result hexOrDecimalBigInt 
	err := client.CallContext(context.Background(), &result, "eth_call", map[string]interface{} {
		"to":   tokenContractAddress,
		"input": "0x313ce567",
		}, "latest")
	if err != nil {
		return 0, err
	}

	decimals, err := strconv.ParseFloat(result.String(), 64)
	if err != nil {
		return decimals, fmt.Errorf("Failed to convert balance to float64")
	}
	return decimals, nil
}

func readConfig() (error) {
	configFile, err := os.Open("config.json")
    if err != nil {
        return fmt.Errorf("Cannot open config.json: %v\n", err)
    }
    defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)

	err = jsonParser.Decode(&config)
	if err != nil {
		return fmt.Errorf("Cannot decode config.json: %v\n", err)
	}

	if err = checkMissingFields(); err != nil {
        return err
    }
    return nil
}

func sendAlert(targets []WalletDetail) (error) {
    bot, err := tgbotapi.NewBotAPI(*config.BotApiKey)
    if err != nil {
        return fmt.Errorf("Failed to create BotAPI instance: %v\n", err)
    }
    bot.Debug = true
    updateConfig := tgbotapi.NewUpdate(0)
    updateConfig.Timeout = 30

    strMessage, entities, err := constructMsg(targets)
    if err != nil {
        return fmt.Errorf("Failed to construct Telegram message: %v\n", err)
    }
    msg := tgbotapi.NewMessage(*config.ChatID, strMessage)
    msg.Entities = entities
    bot.Send(msg)
    return nil
}

func (h *hexOrDecimalBigInt) UnmarshalJSON(data []byte) error {
	str := string(data[1:len(data)-1])

	if str[:2] == "0x" {
		i, success := new(big.Int).SetString(str[2:], 16)
		if !success {
			return fmt.Errorf("Failed to parse hex number")
		}
		h.Int = i
	}
	return nil
}