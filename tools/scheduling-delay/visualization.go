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

func renderChart(delayMaps map[string]map[string]schedulingInfo, manaPercentage map[string]float64) {
	// set xAxis
	for nodeID := range delayMaps {
		xAxis = append(xAxis, nodeID)
	}

	page := components.NewPage()
	page.AddCharts(
		schedulingDelayLineChart(delayMaps, manaPercentage),
		nodeQSizeLineChart(delayMaps, manaPercentage),
	)
	f, err := os.Create("./bar.html")
	if err != nil {
		panic(err)
	}
	page.Render(io.MultiWriter(f))
}

func schedulingDelayLineChart(delayMaps map[string]map[string]schedulingInfo, manaPercentage map[string]float64) *charts.Line {
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

	lineItems := schedulingDelayLineItems(delayMaps)
	for nodeID, items := range lineItems {
		line.AddSeries(nodeID, items)
	}

	line.Overlap(manaBarChart(manaPercentage))

	return line
}

func nodeQSizeLineChart(delayMaps map[string]map[string]schedulingInfo, manaPercentage map[string]float64) *charts.Line {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{Title: "The NodeQueue Size of Each Issuer per Node"}),
		charts.WithXAxisOpts(opts.XAxis{
			Name: "NodeID",
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name:      "buffered bytes in nodeQ",
			AxisLabel: &opts.AxisLabel{Show: true, Formatter: "{value} bytes"},
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
					Title: "Download png file",
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

	lineItems := nodeQueueSizeLineItems(delayMaps)
	for nodeID, items := range lineItems {
		line.AddSeries(nodeID, items)
	}

	line.Overlap(manaBarChart(manaPercentage))
	line.Overlap(scheduledMsgBarChart(delayMaps))

	return line
}

func schedulingDelayLineItems(delayMaps map[string]map[string]schedulingInfo) map[string][]opts.LineData {
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

func nodeQueueSizeLineItems(delayMaps map[string]map[string]schedulingInfo) map[string][]opts.LineData {
	items := make(map[string][]opts.LineData, len(xAxis))
	for _, issuer := range xAxis {
		for _, nodeID := range xAxis {
			items[issuer] = append(items[issuer],
				opts.LineData{Value: delayMaps[nodeID][issuer].nodeQLen})
		}
	}
	return items
}

func manaBarChart(manaMap map[string]float64) *charts.Bar {
	bar := charts.NewBar()
	items := []opts.BarData{}
	for _, issuer := range xAxis {
		mana, ok := manaMap[issuer]
		if !ok {
			mana = 0
		}
		items = append(items, opts.BarData{Value: math.Round(mana * 100)})
	}

	bar.SetXAxis(xAxis).
		AddSeries("mana percentage", items).
		SetSeriesOptions(
			charts.WithLabelOpts(opts.Label{
				Show:      true,
				Position:  "insideBottom",
				Formatter: "{c} %",
			}),
		)
	return bar
}

func scheduledMsgBarChart(delayMaps map[string]map[string]schedulingInfo) *charts.Bar {
	bar := charts.NewBar()
	items := []opts.BarData{}
	for _, issuer := range xAxis {
		items = append(items, opts.BarData{Value: delayMaps[xAxis[0]][issuer].scheduledMsgs})
	}

	bar.SetXAxis(xAxis).
		AddSeries("# of scheduled msgs", items).
		SetSeriesOptions(
			charts.WithLabelOpts(opts.Label{
				Show:      true,
				Position:  "insideBottom",
				Formatter: "{c} msgs",
			}),
		)
	return bar
}
