package main

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/doneland/yquotes"
	"github.com/howeyc/ledger"
	"github.com/howeyc/ledger/cmd/lweb/internal/ghist"
	"github.com/lucasb-eyer/go-colorful"
)

type portfolioValue struct {
	Date          time.Time
	SectionValues map[string]float64
}

func getPortfolioHistoryData() ([]portfolioValue, error) {
	pvs := make([]portfolioValue, 30)
	for _, stock := range stockConfigData.Stocks {
		prices, herr := ghist.Get(stock.Ticker)
		if herr != nil {
			continue
			fmt.Println(stock.Ticker, herr)
		}
		for i, price := range prices[:30] {
			pvs[i].Date = price.Date
			if pvs[i].SectionValues == nil {
				pvs[i].SectionValues = make(map[string]float64)
			}
			val := pvs[i].SectionValues[stock.Section]
			val += stock.Shares * price.Close
			pvs[i].SectionValues[stock.Section] = val
		}
	}
	return pvs, nil
}

func portfolioHandler(w http.ResponseWriter, r *http.Request) {
	t, err := parseAssets("templates/template.portfolio.html", "templates/template.nav.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	trans, terr := getTransactions()
	if terr != nil {
		http.Error(w, terr.Error(), 500)
		return
	}
	balances := ledger.GetBalances(trans, []string{})

	type lineData struct {
		SectionName string
		RGBColor    string
		Values      []float64
	}
	type portPageData struct {
		pageData
		Labels   []string
		DataSets []lineData
	}
	var pData portPageData
	pData.Reports = reportConfigData.Reports
	pData.Transactions = trans

	sectionTotals := make(map[string]stockInfo)
	siChan := make(chan stockInfo)

	for _, stock := range stockConfigData.Stocks {
		go func(name, account, symbol, section string, shares float64) {
			quote, _ := yquotes.GetPrice(symbol)
			var sprice float64
			var sclose float64
			var cprice float64
			if quote != nil {
				sprice = quote.Last
				sclose = quote.PreviousClose
			}
			si := stockInfo{Name: name,
				Section: section,
				Ticker:  symbol,
				Price:   sprice,
				Shares:  shares}
			for _, bal := range balances {
				if account == bal.Name {
					si.Cost, _ = bal.Balance.Float64()
				}
			}
			cprice = si.Cost / si.Shares
			si.MarketValue = si.Shares * si.Price
			si.GainLossOverall = si.MarketValue - si.Cost
			si.PriceChangeDay = sprice - sclose
			si.PriceChangePctDay = (si.PriceChangeDay / sclose) * 100.0
			si.PriceChangeOverall = sprice - cprice
			si.PriceChangePctOverall = (si.PriceChangeOverall / cprice) * 100.0
			si.GainLossDay = si.Shares * si.PriceChangeDay
			siChan <- si
		}(stock.Name, stock.Account, stock.Ticker, stock.Section, stock.Shares)
	}
	for range stockConfigData.Stocks {
		pData.Stocks = append(pData.Stocks, <-siChan)
	}

	stotal := stockInfo{Name: "Total", Section: "Total", Type: "Total"}
	for _, si := range pData.Stocks {
		sectionInfo := sectionTotals[si.Section]
		sectionInfo.Name = si.Section
		sectionInfo.Section = si.Section
		sectionInfo.Type = "Section Total"
		sectionInfo.Ticker = "zzz"
		sectionInfo.Cost += si.Cost
		sectionInfo.MarketValue += si.MarketValue
		sectionInfo.GainLossOverall += si.GainLossOverall
		sectionInfo.GainLossDay += si.GainLossDay
		sectionTotals[si.Section] = sectionInfo

		stotal.Cost += si.Cost
		stotal.MarketValue += si.MarketValue
		stotal.GainLossOverall += si.GainLossOverall
		stotal.GainLossDay += si.GainLossDay
	}
	stotal.PriceChangePctDay = (stotal.GainLossDay / stotal.Cost) * 100.0
	stotal.PriceChangePctOverall = (stotal.GainLossOverall / stotal.Cost) * 100.0
	pData.Stocks = append(pData.Stocks, stotal)

	for _, sectionInfo := range sectionTotals {
		sectionInfo.PriceChangePctDay = (sectionInfo.GainLossDay / sectionInfo.Cost) * 100.0
		sectionInfo.PriceChangePctOverall = (sectionInfo.GainLossOverall / sectionInfo.Cost) * 100.0
		pData.Stocks = append(pData.Stocks, sectionInfo)
	}

	sort.Slice(pData.Stocks, func(i, j int) bool {
		return pData.Stocks[i].Ticker < pData.Stocks[j].Ticker
	})
	sort.SliceStable(pData.Stocks, func(i, j int) bool {
		return pData.Stocks[i].Section < pData.Stocks[j].Section
	})

	colorPalette := colorful.FastHappyPalette(len(sectionTotals) + 1)
	var colorIdx int
	for secName := range sectionTotals {
		r, g, b := colorPalette[colorIdx].RGB255()
		pData.DataSets = append(pData.DataSets,
			lineData{SectionName: secName,
				RGBColor: fmt.Sprintf("%d, %d, %d", r, g, b)})
		colorIdx++
	}
	rc, gc, bc := colorPalette[colorIdx].RGB255()
	pData.DataSets = append(pData.DataSets,
		lineData{SectionName: "Total",
			RGBColor: fmt.Sprintf("%d, %d, %d", rc, gc, bc)})

	pvs, perr := getPortfolioHistoryData()
	if perr != nil {
		http.Error(w, perr.Error(), 500)
		return
	}

	for _, pv := range pvs {
		pData.Labels = append(pData.Labels, pv.Date.Format("2006-01-02"))
		for dIdx := range pData.DataSets {
			val := pv.SectionValues[pData.DataSets[dIdx].SectionName]
			if pData.DataSets[dIdx].SectionName == "Total" {
				for _, v := range pv.SectionValues {
					val += v
				}
			}
			pData.DataSets[dIdx].Values = append(pData.DataSets[dIdx].Values, val)
		}

	}

	err = t.Execute(w, pData)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
}
