package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"time"
	"flag"

	"github.com/asticode/go-astisub"
	"github.com/jftuga/geodist"
)

var (
	// Splits each telemetry value from timingsRe
	telemetryRe = regexp.MustCompile(`(\w+):\s*([^\s\],]+)`)
	timeformat  = "2006-01-02 15:04:05.999" // 2026-08-23 13:00:00.224
)

// djiMeta tracks data the SRT data
type djiMeta struct {
	// Everything following here is directly from or calculated from line 0:
	// <font size="28">FrameCnt: 1, DiffTime: 16ms
	FrameCount  int            `json:"frameCount"`
	DiffTime    string         `json:"diffTime"`
	DiffTimeDur *time.Duration `json:"-"`

	// Everything following here is directly from or calculated from line 1:
	// 2026-08-23 13:00:00.224
	Timestamp *time.Time `json:"timestamp"`

	// Everything following here is directly from or calculated from line 2:
	// [iso: 160] [shutter: 1/640.0] [fnum: 2.8] [ev: 0] [color_md: default] [focal_len: 28.00] [latitude: 37.330244] [longitude: -91.880935] [rel_alt: 2.398 abs_alt: 344.855] [ct: 4199, tint: 9]
	Iso         int     `json:"iso"`
	Shutter     string  `json:"shutter`
	Fstop       string  `json:"fstop"`
	Ev          string  `json:"ev"`
	ColorMode   string  `json:"colorMode"`
	FocalLength float64 `json:"focalLength`
	ColorTemp   int     `json:"colorTemp"`
	Tint        int     `json:"tint"`

	Lat               float64       `json:"lat"`
	Lon               float64       `json:"lon"`
	Point             geodist.Coord `json:"-"`
	AltRelativeMeters float64       `json:"altRelativeMeters"`
	AltRelativeFeet   float64       `json:"altRelativeFeet"` // Calculated from AltRelativeMeters
	AltSeaMeters      float64       `json:"altSeaMeters"`
	AltSeaFeet        float64       `json:"altSeaFeet"` // Calculated from AltSeaMeters

	// To be calculated after all the metadata is parsed
	SpeedHorKm    float64 `json:"speedHorKm"`
	SpeedHorMiles float64 `json:"speedHorMiles"`
	SpeedVerKm    float64 `json:"speedVerKm"`
	SpeedVerMiles float64 `json:"speedVerMiles"`
}

func parseDjiMetaLine(lines []astisub.Line) djiMeta {
	dm := djiMeta{}

	for _, line := range lines {
		for _, lineItem := range line.Items {
			// Try to format as date, if we succeed, save it and continue. Otherwise try to parse it another way
			t, err := time.Parse(timeformat, lineItem.Text)
			if err == nil {
				dm.Timestamp = &t
				continue
			}

			matches := telemetryRe.FindAllStringSubmatch(lineItem.Text, -1)

			for _, m := range matches {
				tag := m[1]
				valueStr := m[2]
				valueInt, intErr := strconv.Atoi(valueStr)
				valueFloat, floatErr := strconv.ParseFloat(valueStr, 64)

				switch tag {
				case "FrameCnt":
					metaSet(&dm.FrameCount, valueInt, intErr)
				case "DiffTime":
					dm.DiffTime = valueStr
					dur, err := time.ParseDuration(valueStr)
					if err == nil {
						dm.DiffTimeDur = &dur
					}
				case "iso":
					metaSet(&dm.Iso, valueInt, intErr)
				case "shutter":
					metaSet(&dm.Shutter, valueStr, nil)
				case "fnum":
					metaSet(&dm.Fstop, valueStr, nil)
				case "ev":
					metaSet(&dm.Ev, valueStr, nil)
				case "color_md":
					metaSet(&dm.ColorMode, valueStr, nil)
				case "focal_len":
					metaSet(&dm.FocalLength, valueFloat, floatErr)
				case "latitude":
					metaSet(&dm.Lat, valueFloat, floatErr)
					dm.Point.Lat = dm.Lat
				case "longitude":
					metaSet(&dm.Lon, valueFloat, floatErr)
					dm.Point.Lon = dm.Lon
				case "rel_alt":
					metaSet(&dm.AltRelativeMeters, valueFloat, floatErr)
					dm.AltRelativeFeet = convertMetersToFeet(dm.AltRelativeMeters)
				case "abs_alt":
					metaSet(&dm.AltSeaMeters, valueFloat, floatErr)
					dm.AltSeaFeet = convertMetersToFeet(dm.AltSeaMeters)
				case "ct":
					metaSet(&dm.ColorTemp, valueInt, intErr)
				case "tint":
					metaSet(&dm.Tint, valueInt, intErr)

				default:
					log.Printf("Warning: found unknown metadata tag, skipping it: %s\n", tag)
				}
			}
		}
	}

	return dm
}

func metaSet[T any](target *T, value T, err error) {
	if err != nil {
		log.Printf("Warning: failed to parse value to expected type: %s\n", err)
	}
	*target = value
}

func convertMetersToFeet(m float64) float64 {
	return m * 3.28084
}
func convertKmHToMph(m float64) float64 {
	return m * 0.621371
}

func metaEqual(a, b djiMeta) bool {
	if a.Lat != b.Lat {
		return false
	}

	if a.Lon != b.Lon {
		return false
	}

	if a.AltRelativeMeters != b.AltRelativeMeters {
		return false
	}

	if a.AltSeaMeters != b.AltSeaMeters {
		return false
	}

	// TODO CLI flag to check other fields?

	return true
}

var (
	rollingAvgCount = flag.Int("avg", 5, "rolling average of h/v speeds")
	rollingAvgSpeedHorKm []float64
	rollingAvgSpeedHorMiles []float64
	rollingAvgSpeedVerKm []float64
	rollingAvgSpeedVerMiles []float64
)

func (m *djiMeta) Speed(from djiMeta) {
	if from.Timestamp == nil || m.Timestamp == nil {
		log.Printf("Warning: failed to calculate speed, one or both points missing timestamp\n")
		return
	}

	miles, km, err := geodist.VincentyDistance(from.Point, m.Point)
	if err != nil {
		log.Printf("Warning: failed to calculate the distance between two GPS points%s\n", err)
		return
	}

	timePassed := from.Timestamp.Sub(*m.Timestamp)
	log.Printf("m.Timestamp.Sub(*from.Timestamp) = %s\n", timePassed)

	m.SpeedHorKm = km / timePassed.Hours()
	rollingAvgSpeedHorKm = append(rollingAvgSpeedHorKm, m.SpeedHorKm)
	if len(rollingAvgSpeedHorKm) > *rollingAvgCount {
		rollingAvgSpeedHorKm = rollingAvgSpeedHorKm[1:]
	}
	temp := 0.0
	for _, s := range rollingAvgSpeedHorKm{
		temp += s
	}
	m.SpeedHorKm = temp / float64(len(rollingAvgSpeedHorKm))

	m.SpeedHorMiles = miles / timePassed.Hours()
	rollingAvgSpeedHorMiles = append(rollingAvgSpeedHorMiles, m.SpeedHorMiles)
	if len(rollingAvgSpeedHorMiles) > *rollingAvgCount {
		rollingAvgSpeedHorMiles = rollingAvgSpeedHorMiles[1:]
	}
	temp = 0.0
	for _, s := range rollingAvgSpeedHorMiles{
		temp += s
	}
	m.SpeedHorMiles = temp / float64(len(rollingAvgSpeedHorMiles))

	vertDiffMeters := from.AltSeaMeters - m.AltSeaMeters
	m.SpeedVerKm = (vertDiffMeters / 1000.0) / timePassed.Hours()
	rollingAvgSpeedVerKm = append(rollingAvgSpeedVerKm, m.SpeedVerKm)
	if len(rollingAvgSpeedVerKm) > *rollingAvgCount {
		rollingAvgSpeedVerKm = rollingAvgSpeedVerKm[1:]
	}
	temp = 0.0
	for _, s := range rollingAvgSpeedVerKm{
		temp += s
	}
	m.SpeedVerKm = temp / float64(len(rollingAvgSpeedVerKm))

	m.SpeedVerMiles = convertKmHToMph(m.SpeedVerKm)
	rollingAvgSpeedVerMiles = append(rollingAvgSpeedVerMiles, m.SpeedVerMiles)
	if len(rollingAvgSpeedVerMiles) > *rollingAvgCount {
		rollingAvgSpeedVerMiles = rollingAvgSpeedVerMiles[1:]
	}
	temp = 0.0
	for _, s := range rollingAvgSpeedVerMiles{
		temp += s
	}
	m.SpeedVerMiles = temp / float64(len(rollingAvgSpeedVerMiles))
}

var (
	noOutputPos = flag.Bool("no-pos", false, "don't output lat/long in thinned SRT")
	noMetric = flag.Bool("no-metric", true, "don't output metric values; WTF is a kilometer?")
	noImperial = flag.Bool("no-imperial", false, "don't output imperial values")
)

func (m *djiMeta) OutputLine() string {
	result := ""

	if !*noOutputPos {
		result += fmt.Sprintf("latitude: %f, longitude: %f", m.Lat, m.Lon)
	}

	if !*noMetric {
		result += fmt.Sprintf("h_kmh: %0.1f, v_kmh: %0.1f", m.SpeedHorKm, m.SpeedVerKm)
	}

	if !*noImperial {
		result += fmt.Sprintf("h_mph: %0.1f, v_mph: %0.1f", m.SpeedHorMiles, m.SpeedVerMiles)
	}

	return result
}
