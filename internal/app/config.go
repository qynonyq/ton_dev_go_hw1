package app

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const (
	Billion          = 1_000_000_000
	MainnetCfgURL    = "https://ton.org/global.config.json"
	TestnetCfgURL    = "https://ton-blockchain.github.io/testnet-global.config.json"
	JettonWalletAddr = "EQBZXk6SAkZIwx_vJDQn46PayCOgAAs4lRwhwMc6lehLxTVF"
)

type (
	Cfg struct {
		LogLevel string
		Wallet   Wallet
	}

	Wallet struct {
		Seed []string
	}
)

func initConfig() (*Cfg, error) {
	if err := godotenv.Load(); err != nil {
		return nil, err
	}

	cfg := Cfg{
		LogLevel: os.Getenv("LOG_LEVEL"),
		Wallet: Wallet{
			Seed: strings.Split(os.Getenv("SEED"), " "),
		},
	}

	return &cfg, nil
}
