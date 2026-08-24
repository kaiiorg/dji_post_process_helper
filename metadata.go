package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"

	"github.com/jftuga/geodist"
)

var (
	// Splits each telemetry value from timingsRe
	telemetryRe = regexp.MustCompile(`(\w+):\s*([^\s\],]+)`)
)

// djiMeta tracks data the SRT data
type djiMeta struct {
	// Everything following here is directly from or calculated from
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

func parseDjiMetaLine(line string) djiMeta {
	dm := djiMeta{}

	matches := telemetryRe.FindAllStringSubmatch(line, -1)

	for _, m := range matches {
		tag := m[1]
		valueStr := m[2]
		valueInt, intErr := strconv.Atoi(valueStr)
		valueFloat, floatErr := strconv.ParseFloat(valueStr, 64)

		switch tag {
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

func (m *djiMeta) Speed(from djiMeta) {
	miles, km, err := geodist.VincentyDistance(from.Point, m.Point)
	if err != nil {
		log.Printf("Warning: failed to calculate the distance between two GPS points%s\n", err)
		return
	}

	vertDiffMeters := from.AltSeaMeters - m.AltSeaMeters

	// TODO this is distance, not speed! We don't have a timeframe yet!
	m.SpeedHorKm = km
	m.SpeedHorMiles = miles
	// TODO this is distance, not speed! We don't have a timeframe yet! And it is in meters/feet
	m.SpeedVerKm = vertDiffMeters
	m.SpeedVerMiles = convertMetersToFeet(vertDiffMeters)
}

func (m *djiMeta) OutputLine() string {
	result := fmt.Sprintf(
		"latitude: %f, longitude: %f, hkm_s: %f, hm_h: %f, vkm_s: %f, vm_h: %f",
		m.Lat,
		m.Lon,
		m.SpeedHorKm,
		m.SpeedHorMiles,
		m.SpeedVerKm,
		m.SpeedVerMiles,
	)

	return result
}
