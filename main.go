package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type binanceResp struct {
	Price float64 `json:"price,string"`
	Code  int64   `json:"code"`
}

type wallet map[string]float64

type db struct {
	wallets map[int64]wallet
}

func NewDb() *db {
	return &db{
		wallets: map[int64]wallet{},
	}
}

func (db *db) Operation(chatID int64, currency string, value string, operationFunc func(float64, float64) (float64, error)) string {
	summ, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return err.Error()
	}

	if _, ok := db.wallets[chatID]; !ok {
		db.wallets[chatID] = wallet{}
	}

	db.wallets[chatID][currency], err = operationFunc(db.wallets[chatID][currency], summ)
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("Balance: %s %f", currency, db.wallets[chatID][currency])
}

func (db *db) Delete(chatID int64, currency string) string {
	delete(db.wallets[chatID], currency)
	return "currency deleted"
}

func (db *db) Show(chatID int64) string {
	result := "Balance:\n"
	var usdSumm float64
	var rubSumm float64

	rubPrice, err := getPrice("USDTRUB")
	if err != nil {
		return err.Error()
	}

	for key, value := range db.wallets[chatID] {
		usdPrice, err := getPrice(key + "USDT")
		if err != nil {
			return err.Error()
		}

		usdSumm += value * usdPrice
		rubSumm += value * usdPrice * rubPrice
		result += fmt.Sprintf("%s: %f [%.2f USD %.2f RUB]\n", key, value, value*usdPrice, value*usdPrice*rubPrice)
	}

	result += "Total:\n"
	result += fmt.Sprintf("%f USD\n", usdSumm)
	result += fmt.Sprintf("%f RUB\n", rubSumm)
	return result
}

func main() {
	bot, err := tgbotapi.NewBotAPI(getKey())
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	db := NewDb()
	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Panic(err)
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}

		log.Println(update.Message.Text)

		msgArr := strings.Split(update.Message.Text, " ")
		var msg string
		switch msgArr[0] {
		case "ADD":
			msg = db.Operation(
				update.Message.Chat.ID, msgArr[1], msgArr[2],
				func(a float64, b float64) (float64, error) {
					return a + b, nil
				})
		case "SUB":
			msg = db.Operation(
				update.Message.Chat.ID, msgArr[1], msgArr[2],
				func(a float64, b float64) (float64, error) {
					if a < b {
						return 0, errors.New("not enough funds on balance")
					}
					return a - b, nil
				})
		case "DEL":
			msg = db.Delete(update.Message.Chat.ID, msgArr[1])
		case "SHOW":
			msg = db.Show(update.Message.Chat.ID)
		default:
			msg = "unsupported operation"
		}

		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, msg))
	}
}

func getKey() string {
	return ""
}

func getPrice(pair string) (price float64, err error) {
	resp, err := http.Get(fmt.Sprintf("https://api.binance.com/api/v3/ticker/price?symbol=%s", pair))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var jsonResp binanceResp

	err = json.NewDecoder(resp.Body).Decode(&jsonResp)
	if err != nil {
		return
	}

	if jsonResp.Code != 0 {
		err = errors.New("uncorrect currency")
		return
	}

	price = jsonResp.Price

	return
}
