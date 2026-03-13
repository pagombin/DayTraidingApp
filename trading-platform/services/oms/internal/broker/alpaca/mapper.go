package alpaca

import (
	"trading-platform/oms/internal/model"
)

func MapOrderType(ot model.OrderType) string {
	switch ot {
	case model.OrderTypeMarket:
		return "market"
	case model.OrderTypeLimit:
		return "limit"
	case model.OrderTypeStop:
		return "stop"
	case model.OrderTypeStopLimit:
		return "stop_limit"
	case model.OrderTypeTrailingStop:
		return "trailing_stop"
	default:
		return "market"
	}
}

func MapTimeInForce(tif model.TimeInForce) string {
	switch tif {
	case model.TIFDay:
		return "day"
	case model.TIFGTC:
		return "gtc"
	case model.TIFIOC:
		return "ioc"
	case model.TIFFOK:
		return "fok"
	default:
		return "day"
	}
}
