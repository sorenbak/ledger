package ghist

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"time"
)

// Price is the open and closing prices for a the specified date
type Price struct {
	Date        time.Time
	Open, Close float64
}

// Get will return the history for a given ticker symbol
func Get(symbol string) ([]Price, error) {
	url := "https://www.google.com/finance/historical?output=csv&q=" + symbol
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	csvr := csv.NewReader(resp.Body)
	records, rerr := csvr.ReadAll()
	if rerr != nil {
		return nil, rerr
	}

	prices := make([]Price, len(records)-1)
	for i, record := range records[1:] {
		prices[i].Date, _ = time.Parse("2-Jan-06", record[0])
		prices[i].Open, _ = strconv.ParseFloat(record[1], 64)
		prices[i].Close, _ = strconv.ParseFloat(record[4], 64)
	}
	return prices, nil
}
