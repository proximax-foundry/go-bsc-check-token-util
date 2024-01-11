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
	"encoding/hex"
	"encoding/json"
	"github.com/ethereum/go-ethereum/rpc"
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Config struct {
    BotApiKey     			*string
    ChatID        			*int64
    Quantity     			*float64
    Sleep         			*int
    WalletAddress      		[]*string
    TokenContractAddress	[]*string
}

type hexOrDecimalBigInt struct {
	*big.Int
}

type Symbol struct {
	Name string
}

type WalletDetail struct {
    Address		string
    Tokens		[]string
}

var config Config
var quantity *big.Float

func main() {
	err := readConfig()
    if err != nil {
        errHandling(err)
    }

	client, err := rpc.Dial("https://bsc-dataseed.binance.org/")
	if err != nil {
		errHandling(fmt.Errorf("Failed to send RPC call: %v\n", err))
	}
	defer client.Close()

	quantity = big.NewFloat(*config.Quantity)

	for {
		var targets []WalletDetail 
		for _, walletAddress := range config.WalletAddress {
			var tokens []string
			bnbBalance, err := getBNBBalance(client, *walletAddress)
			if err != nil {
				errHandling(fmt.Errorf("Failed to get BNB balance: %v\n", err))
			}

			if bnbBalance.Cmp(quantity) < 0 {
				tokens = append(tokens, "BNB")
			}

			for _, tokenContractAddress := range config.TokenContractAddress {
				tokenBalance, symbol, err := getTokenBalance(client, *tokenContractAddress, *walletAddress)
				if err != nil {
					errHandling(fmt.Errorf("Failed to get token balance: %v\n", err))
				}
				if tokenBalance.Cmp(quantity) < 0 {
					tokens = append(tokens, symbol)
				}
			}
			if len(tokens) > 0 {
				targets = append(targets, WalletDetail{
					Address: *walletAddress,
					Tokens: tokens,
				})
			}
		}
		if len(targets) > 0 {
			err := sendAlert(targets)
			if err != nil {
                errHandling(fmt.Errorf("Failed to send alert: %v\n", err))
            }
		}
		time.Sleep(time.Duration(*config.Sleep) * time.Second)
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
    strMsg := "The following BSC accounts' tokens have balance less than " + quantity.String() + ": \n\n"
    strMsgLen := len(strMsg)
    entities := []tgbotapi.MessageEntity{}
    for i, target := range targets {
        entities = append(entities, tgbotapi.MessageEntity{
            Type: "text_link",
            Offset: i*43 + n + strMsgLen,
            Length: 42,
            URL: "https://bscscan.com/address/" + target.Address,
        })

        strMsg += target.Address + "\n"
        for _, token := range target.Tokens {
            strMsg += "- " + token + "\n"
            n += len(token) + 3
        }
    }
    return strMsg, entities, nil
}

func errHandling(err error) {
	log.SetOutput(os.Stderr)
    log.Fatal("Error: ", err)
}

func getBNBBalance(client *rpc.Client, address string) (*big.Float, error) {
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

func getTokenBalance(client *rpc.Client, tokenContractAddress string, address string) (*big.Float, string, error) {
	data := fmt.Sprintf("0x70a08231000000000000000000000000%s", address[2:])

	var result hexOrDecimalBigInt
	err := client.CallContext(context.Background(), &result, "eth_call", map[string]interface{} {
		"to":   tokenContractAddress,
		"input": data,
		}, "latest")
	if err != nil {
		return nil, "", err
	}

	balance, success := new(big.Int).SetString(result.String(), 0)
	if !success {
		return nil, "", fmt.Errorf("Failed to convert balance to big.Int")
	}

	decimals, err := getTokenDecimals(client, tokenContractAddress)
	if err != nil {
		return nil, "", fmt.Errorf("Failed to get token decimals: %v\n", err)
	}
	symbol, err := getTokenSymbol(client, tokenContractAddress)
	if err != nil {
		return nil, "", fmt.Errorf("Failed to get token symbol: %v\n", err)
	}
	balanceFloat := new(big.Float).Quo(new(big.Float).SetInt(balance), big.NewFloat(math.Pow(10, decimals)))
	return balanceFloat, symbol, nil
}

func getTokenDecimals(client *rpc.Client, tokenContractAddress string) (float64, error) {
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

func getTokenSymbol(client *rpc.Client, tokenContractAddress string) (string, error) {
	var result Symbol 
	err := client.CallContext(context.Background(), &result, "eth_call", map[string]interface{} {
		"to":   tokenContractAddress,
		"input": "0x95d89b41",
		}, "latest")
	if err != nil {
		return "", err
	}
	return result.Name, nil
}

func readConfig() (error) {
    configFile, err := os.Open("config.json")

    if err != nil {
        return fmt.Errorf("Cannot open config.json: %v\n", err)
    }

    defer configFile.Close()

    jsonParser := json.NewDecoder(configFile)
    if jsonParser.Decode(&config); err != nil {
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
		// Hex
		i, success := new(big.Int).SetString(str[2:], 16)
		if !success {
			return fmt.Errorf("Failed to parse hex number")
		}
		h.Int = i
	} else {
		// Decimal
		i, success := new(big.Int).SetString(str, 10)
		if !success {
			return fmt.Errorf("Failed to parse decimal number")
		}
		h.Int = i
	}
	return nil
}

func (s *Symbol) UnmarshalJSON(data []byte) error {
	str := string(data[1:len(data)-1])

	bytes, err := hex.DecodeString(str[130:])
	if err != nil {
		return err
	}
	s.Name = strings.TrimSpace(string(bytes))
	return nil
}