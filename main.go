package main 

import (
	"flag"
	"log"
	"os"
	"path/filepath"
)

var (
	basePath    = flag.String("b", "./", "base path where all expected files live")
	inBaseName  = flag.String("i", "", "Original base name used by all files, such as DJI_20260823130819_0051_D")
	outBaseName = flag.String("o", "", "New base name to be used by all files")
)

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

func main() {
	flag.Parse()

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

