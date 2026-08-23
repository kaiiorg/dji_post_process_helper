build:
	go build -o ./bin

fmt:
	gofmt -w -s .

reset:
	rm -f ./testdata/*.lrf
	rm -f ./testdata/*.LRF
	rm -f ./testdata/*.mp4
	rm -f ./testdata/*.MP4
	rm -f ./testdata/*.srt
	rm -f ./testdata/*.SRT
	cp ./testdata/originals/* ./testdata