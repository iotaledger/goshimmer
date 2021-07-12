package main

import (
	"io"
	"math"
	"os"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
)

var xAxis = []string{}

func generateLineItems(delayMaps map[string]map[string]schedulingInfo) map[string][]opts.LineData {
	items := make(map[string][]opts.LineData, len(xAxis))
	for _, issuer := range xAxis {
		for _, nodeID := range xAxis {
			delay := time.Duration(delayMaps[nodeID][issuer].avgDelay) * time.Nanosecond
			items[issuer] = append(items[issuer],
				opts.LineData{Value: delay.Milliseconds()})
		}
	}
	return items
}

func generateBarItems(manaMap map[string]float64) []opts.BarData {
	items := []opts.BarData{}
	for _, issuer := range xAxis {
		mana, ok := manaMap[issuer]
		if !ok {
			mana = 0
		}
		items = append(items, opts.BarData{Value: math.Round(mana * 100)})
	}
	return items
}

func manaBarChart(manaMap map[string]float64) *charts.Bar {
	bar := charts.NewBar()
	bar.SetXAxis(xAxis).
		AddSeries("mana percentage", generateBarItems(manaMap)).
		SetSeriesOptions(
			charts.WithLabelOpts(opts.Label{
				Show:      true,
				Position:  "inside",
				Formatter: "{c} %",
			}),
		)
	return bar
}

func generateBarChart(delayMaps map[string]map[string]schedulingInfo, manaPercentage map[string]float64) *charts.Line {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{Title: "Scheduling Delay of Each Issuer per Node"}),
		charts.WithXAxisOpts(opts.XAxis{
			Name: "NodeID",
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name:      "Avg Scheduling Delay",
			AxisLabel: &opts.AxisLabel{Show: true, Formatter: "{value} ms"},
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: true}),
		charts.WithLegendOpts(opts.Legend{
			Show:   true,
			Right:  "1%",
			Top:    "10%",
			Orient: "vertical",
		}),
		charts.WithToolboxOpts(opts.Toolbox{
			Show:  true,
			Right: "5%",
			Feature: &opts.ToolBoxFeature{
				SaveAsImage: &opts.ToolBoxFeatureSaveAsImage{
					Show:  true,
					Type:  "png",
					Title: "Anything you want",
				},
				DataView: &opts.ToolBoxFeatureDataView{
					Show:  true,
					Title: "DataView",
					// set the language
					// Chinese version: ["数据视图", "关闭", "刷新"]
					Lang: []string{"data view", "turn off", "refresh"},
				},
			}}),
	)
	line.SetXAxis(xAxis)

	lineItems := generateLineItems(delayMaps)
	for nodeID, items := range lineItems {
		line.AddSeries(nodeID, items)
	}

	line.Overlap(manaBarChart(manaPercentage))

	return line
}

func renderChart(delayMaps map[string]map[string]schedulingInfo, manaPercentage map[string]float64) {
	// set xAxis
	for nodeID := range delayMaps {
		xAxis = append(xAxis, nodeID)
	}

	page := components.NewPage()
	page.AddCharts(
		generateBarChart(delayMaps, manaPercentage),
	)
	f, err := os.Create("./bar.html")
	if err != nil {
		panic(err)
	}
	page.Render(io.MultiWriter(f))
}
