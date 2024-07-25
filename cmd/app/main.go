package main

import (
	"context"
	"log"
	"strconv"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"

	"github.com/qynonyq/ton_dev_go_hw1/internal/app"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	a, err := app.InitApp()
	if err != nil {
		return err
	}
	ctx := context.Background()

	jettonWallet, err := address.ParseAddr(app.JettonWalletAddr)
	if err != nil {
		return err
	}

	client := liteclient.NewConnectionPool()
	if err := client.AddConnectionsFromConfigUrl(ctx, app.TestnetCfgURL); err != nil {
		return err
	}

	newApi := ton.NewAPIClient(client)
	api := newApi.WithRetry(5)

	w, err := wallet.FromSeed(api, a.Cfg.Wallet.Seed, wallet.V4R2)
	if err != nil {
		return err
	}

	lastMaster, err := api.CurrentMasterchainInfo(ctx)
	if err != nil {
		return err
	}

	addr := w.WalletAddress().Testnet(true)
	acc, err := api.GetAccount(ctx, lastMaster, addr)
	if err != nil {
		return err
	}
	lastLt := acc.LastTxLT
	id := uuid.New().String()
	logrus.Infof("ID for transaction: %s", id)

	txCh := make(chan *tlb.Transaction)
	go api.SubscribeOnTransactions(ctx, addr, lastLt, txCh)

	logrus.Infof("start checking transactions on address %s", addr)
	for {
		select {
		case tx := <-txCh:
			if tx.IO.In.MsgType != tlb.MsgTypeInternal {
				logrus.Warn("not internal message")
				continue
			}

			internalMsg := tx.IO.In.AsInternal()
			if internalMsg.Body == nil {
				logrus.Warn("empty body")
				continue
			}

			bodySlice := internalMsg.Body.BeginParse()
			amount := internalMsg.Amount.String()

			opcode, err := bodySlice.LoadUInt(32)
			if err != nil {
				logrus.Infof("no opcode, %s TON received", amount)
				continue
			}

			switch opcode {
			// text comment
			case 0:
				comment, err := bodySlice.LoadStringSnake()
				if err != nil {
					logrus.Errorf("failed to parse comment: %s", err)
					continue
				}
				if comment == id {
					logrus.Infof("successful top up (%s TON), user ID: %s", amount, id)
				} else {
					logrus.Warnf("unsuccessful top up, %s TON received, comment: %s", amount, comment)
				}
			// jetton transfer notification
			case 0x7362d09c:
				srcAddr := internalMsg.SrcAddr.String()
				if srcAddr != jettonWallet.String() {
					logrus.Warnf("unknown jetton wallet: %s", srcAddr)
					continue
				}

				queryID, err := bodySlice.LoadUInt(64)
				if err != nil {
					logrus.Errorf("failed to parse query_id: %s", err)
					continue
				}

				amount, err := bodySlice.LoadCoins()
				if err != nil {
					logrus.Errorf("failed to load coins: %s", err)
					continue
				}

				sender, err := bodySlice.LoadAddr()
				if err != nil {
					logrus.Errorf("failed to parse sender address: %s", err)
					continue
				}

				amountFmt := strconv.FormatFloat(float64(amount)/app.Billion, 'f', -1, 32)
				logrus.Infof("[JTN] sender: %s, amount: %s, query_id: %d", sender, amountFmt, queryID)

				fwdPayload, err := bodySlice.LoadMaybeRef()
				if err != nil {
					logrus.Errorf("failed to parse forward payload: %s", err)
					continue
				}

				if fwdPayload != nil {
					opcode, err := fwdPayload.LoadUInt(32)
					if err != nil {
						logrus.Errorf("failed to parse forward payload opcode: %s", err)
						continue
					}
					if opcode != 0 {
						logrus.Warnf("wrong forward payload opcode: %d", opcode)
						continue
					}

					comment, err := fwdPayload.LoadStringSnake()
					if err != nil {
						logrus.Errorf("failed to parse forward payload comment: %s", err)
						continue
					}
					if comment == id {
						logrus.Infof("successful top up (%s JTN), user ID: %s", amountFmt, id)
					} else {
						logrus.Warnf("unsuccessful top up, %s JTN received, comment: %s", amountFmt, comment)
					}
				} else {
					logrus.Infof("[JTN] forward payload is empty")
				}
			default:
				logrus.Warnf("unsupported opcode: %d", opcode)
			}

		}
	}

	return nil
}
