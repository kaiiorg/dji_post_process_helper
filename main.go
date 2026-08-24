package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/asticode/go-astisub"
)

var (
	basePath    = flag.String("b", "./", "base path where all expected files live")
	inBaseName  = flag.String("i", "", "Original base name used by all files, such as DJI_20260823130819_0051_D")
	outBaseName = flag.String("o", "", "New base name to be used by all files")
)

func main() {
	flag.Parse()

	//rename()
	thinSRT()
}

type extentions struct {
	src string
	dst string
}

var (
	expectedExtentions = []extentions{
		{"SRT", "srt"},
		{"LRF", "lrf"},
		{"MP4", "mp4"},
	}
)

func rename() {
	if len(*inBaseName) == 0 {
		log.Fatalln("did not provide original base name")
	}

	if len(*outBaseName) == 0 {
		log.Fatalln("did not provide new base name")
	}

	*basePath = filepath.Clean(*basePath)
	// Check that the directory exists
	info, err := os.Stat(*basePath)
	if err != nil {
		log.Fatalf("failed to find base directory: %s\n", err)
	}
	if !info.IsDir() {
		log.Fatalf("failed to find base directory is not a directory\n")
	}

	// Check expected files exist
	var missing []string
	for _, ext := range expectedExtentions {
		src := filepath.Join(*basePath, *inBaseName+"."+ext.src)
		if _, err := os.Stat(src); err != nil {
			missing = append(missing, src)
		}
	}
	if len(missing) > 0 {
		log.Fatalf("failed to find one or more expected files: %#v\n", missing)
	}

	// Rename
	for _, ext := range expectedExtentions {
		src := filepath.Join(*basePath, *inBaseName+"."+ext.src)
		dst := filepath.Join(*basePath, *outBaseName+"."+ext.dst)

		// Refuse to overwrite an existing destination
		if _, err := os.Stat(dst); err == nil {
			log.Fatalf("failed to rename '%s' to '%s': file at destination already exists\n", src, dst)
		}

		if err := os.Rename(src, dst); err != nil {
			log.Fatalf("failed to rename '%s' to '%s': %s\n", src, dst, err)
		}

		log.Printf("renamed '%s' to '%s'\n", src, dst)
	}
}

func thinSRT() {
	telemetrySubs, err := astisub.OpenFile(filepath.Join(*basePath, *outBaseName+".srt"))
	if err != nil {
		log.Fatalf("failed to open telemetry subtitles: %s\n", err)
	}
	if telemetrySubs.IsEmpty() {
		log.Println("no data in telemetry subtitles")
		return
	}
	log.Printf("opened telemetry subtitles, %d frames\n", len(telemetrySubs.Items))

	thinnedTelemetrySubs := astisub.NewSubtitles()
	lastItem := telemetrySubs.Items[0]
	lastDjiMeta := parseDjiMetaLine(lastItem.Lines[2].Items[0].Text) // TODO need a reliable way of detecting the telemetry line without always assuming it is line 3!

	for _, frame := range telemetrySubs.Items {
		/*
			for l, line := range frame.Lines {
				for li, lineItem := range line.Items {
					log.Printf("%d:%d:%d %s", frame.Index, l, li, lineItem.Text)
				}
			}
		*/

		// TODO need a reliable way of detecting the telemetry line without always assuming it is line 3!
		currentDjiMeta := parseDjiMetaLine(frame.Lines[2].Items[0].Text)

		if metaEqual(lastDjiMeta, currentDjiMeta) {
			log.Printf("%d has duplicate telemetry as %d\n", frame.Index, lastItem.Index)
			continue
		}

		log.Printf("%d has new telemetry\n", frame.Index)
		lastDjiMeta.Speed(currentDjiMeta)
		// Make a continous stream of telemetry by setting the end time of the last item to the start time of the new one
		lastItem.EndAt = frame.StartAt
		lastItem.Lines = []astisub.Line{
			{
				Items: []astisub.LineItem{
					{
						Text: lastDjiMeta.OutputLine(),
					},
				},
			},
		}
		// TODO the DiffTime value on the first line hasn't been updated and will be wrong!

		// Append the last item to the list, then replace it with the new one
		thinnedTelemetrySubs.Items = append(thinnedTelemetrySubs.Items, lastItem)
		lastItem = frame
		lastDjiMeta = currentDjiMeta
	}

	log.Printf("Thinned %d frames down to %d\n", len(telemetrySubs.Items), len(thinnedTelemetrySubs.Items))
	thinnedTelemetrySubs.Metadata = telemetrySubs.Metadata
	thinnedTelemetrySubs.Regions = telemetrySubs.Regions
	thinnedTelemetrySubs.Styles = telemetrySubs.Styles

	f, err := os.Create(filepath.Join(*basePath, *outBaseName+"_thinned.srt"))
	if err != nil {
		log.Fatalf("failed to open file to dump thinned subtitle telemetry to: %s\n", err)
	}
	defer f.Close()

	err = thinnedTelemetrySubs.WriteToSRT(f)
	if err != nil {
		log.Fatalf("failed to write thinned subtitle telemetry as SRT: %s\n", err)
	}
	log.Println("wrote thinned SRT")
}
