package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type OptionType string

const (
	OptionCall OptionType = "call"
	OptionPut  OptionType = "put"
)

type OptionGreeks struct {
	Delta decimal.Decimal `json:"delta"`
	Gamma decimal.Decimal `json:"gamma"`
	Theta decimal.Decimal `json:"theta"`
	Vega  decimal.Decimal `json:"vega"`
	Rho   decimal.Decimal `json:"rho"`
}

type OptionQuote struct {
	ContractSymbol    string          `json:"contract_symbol"`
	Underlying        string          `json:"underlying"`
	Expiration        time.Time       `json:"expiration"`
	Strike            decimal.Decimal `json:"strike"`
	OptionType        OptionType      `json:"option_type"`
	Bid               decimal.Decimal `json:"bid"`
	Ask               decimal.Decimal `json:"ask"`
	Last              decimal.Decimal `json:"last"`
	Volume            int64           `json:"volume"`
	OpenInterest      int64           `json:"open_interest"`
	ImpliedVolatility decimal.Decimal `json:"implied_volatility"`
	Greeks            OptionGreeks    `json:"greeks"`
	Timestamp         time.Time       `json:"timestamp"`
	Source            string          `json:"source"`
}
