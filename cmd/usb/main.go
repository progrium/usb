package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/asticode/go-astisub"
	"github.com/blevesearch/bleve"
	_ "github.com/blevesearch/bleve/analysis/analyzer/simple"
	"github.com/qeesung/image2ascii/convert"
	"gocv.io/x/gocv"
)

// {"episode": "s01e02", "index": 1, "start": "0:00:01.366000", "end": "0:00:05.900000", "milliseconds_middle": 3633.0, "content": "So I'm on a line\nat the supermarket.", "preview": "previews/s01e02_3633.0.png"}

type Subtitle struct {
	Episode string `json:"episode"`
	Season  string `json:"season"`
	Index   int    `json:"index"`
	Start   int64  `json:"start"`
	End     int64  `json:"end"`
	Content string `json:"content"`
	Preview string `json:"preview"`
}

const IndexFile = "./index"

func main() {
	switch os.Args[1] {
	case "reindex":
		os.RemoveAll(IndexFile)

		mapping := bleve.NewIndexMapping()
		mapping.DefaultAnalyzer = "simple"
		index, err := bleve.New(IndexFile, mapping)
		if err != nil {
			panic(err)
		}

		files, err := ioutil.ReadDir("./json")
		if err != nil {
			panic(err)
		}
		for _, file := range files {
			content, err := ioutil.ReadFile("./json/" + file.Name())
			if err != nil {
				panic(err)
			}
			var doc []Subtitle
			err = json.Unmarshal(content, &doc)
			if err != nil {
				panic(err)
			}
			batch := index.NewBatch()
			for i, sub := range doc {
				id := fmt.Sprintf("%s-%d", doc[0].Episode, i)
				fmt.Println(id)
				err = batch.Index(id, sub)
				if err != nil {
					panic(err)
				}
			}
			err = index.Batch(batch)
			if err != nil {
				panic(err)
			}

		}
	case "search":
		index, err := bleve.Open(IndexFile)
		if err != nil {
			panic(err)
		}
		// search for some text
		//query := bleve.NewMatchQuery(os.Args[1])
		query := bleve.NewPhraseQuery(strings.Split(os.Args[2], " "), "content")
		search := bleve.NewSearchRequest(query)
		search.Fields = []string{"content"}
		search.Size = 100
		searchResults, err := index.Search(search)
		if err != nil {
			log.Fatal(err)
		}
		// fmt.Println(searchResults)
		for _, hit := range searchResults.Hits {
			fmt.Println(hit, strings.ReplaceAll(hit.Fields["content"].(string), "\n", " "))
		}

	case "generate":

		epRegex := regexp.MustCompile("^(s\\d{2})((?:e\\d{2})+)$")
		os.MkdirAll("./json", 0755)
		os.MkdirAll("./previews", 0755)

		files, err := ioutil.ReadDir("./subs")
		if err != nil {
			panic(err)
		}
		for _, file := range files {
			if filepath.Ext(file.Name()) != ".srt" {
				continue
			}
			fname := filepath.Base(file.Name())
			epname := strings.TrimSuffix(fname, filepath.Ext(fname))
			res := epRegex.FindAllStringSubmatch(epname, -1)
			season := res[0][1]
			episode := res[0][2]

			srt, err := astisub.OpenFile("./subs/" + file.Name())
			if err != nil {
				panic(err)
			}
			var subs []Subtitle
			video, err := gocv.VideoCaptureFile(fmt.Sprintf("./videos/%s.mkv", epname))
			if err != nil {
				panic(err)
			}
			for idx, sub := range srt.Items {
				previewPath := fmt.Sprintf("./previews/%s_%d.png", epname, idx)
				if _, err := os.Stat(previewPath); os.IsNotExist(err) {
					m := gocv.NewMat()
					video.Set(gocv.VideoCapturePosMsec, float64(sub.StartAt.Milliseconds()))
					if video.Read(&m) {
						if gocv.IMWrite(previewPath, m) {
							fmt.Println(previewPath)
							// converter := convert.NewImageConverter()
							// convertOptions := convert.DefaultOptions
							// convertOptions.FitScreen = true
							// convertOptions.Colored = true
							// asciiString := converter.ImageFile2ASCIIString(previewPath, &convertOptions)
							// fmt.Println(asciiString)
						}
					}
					m.Close()
				}

				doc := Subtitle{
					Episode: episode,
					Season:  season,
					Index:   idx,
					Start:   sub.StartAt.Milliseconds(),
					End:     sub.EndAt.Milliseconds(),
					Content: sub.String(),
					Preview: previewPath,
				}
				subs = append(subs, doc)
			}
			if err := video.Close(); err != nil {
				panic(err)
			}

			b, err := json.Marshal(subs)
			if err != nil {
				panic(err)
			}
			err = ioutil.WriteFile(fmt.Sprintf("./json/%s%s.json", season, episode), b, 0644)
			if err != nil {
				panic(err)
			}
		}

	case "find":
		index, err := bleve.Open(IndexFile)
		if err != nil {
			panic(err)
		}
		query := bleve.NewPhraseQuery(strings.Split(os.Args[2], " "), "content")
		search := bleve.NewSearchRequest(query)
		search.Fields = []string{"content", "preview"}
		search.Size = 1
		searchResults, err := index.Search(search)
		if err != nil {
			log.Fatal(err)
		}
		if len(searchResults.Hits) > 0 {
			hit := searchResults.Hits[0]
			preview := "./" + hit.Fields["preview"].(string)

			if _, err := os.Stat(preview); !os.IsNotExist(err) {
				converter := convert.NewImageConverter()
				imageFilename := preview
				convertOptions := convert.DefaultOptions
				convertOptions.FitScreen = true
				convertOptions.Colored = true
				asciiString := converter.ImageFile2ASCIIString(imageFilename, &convertOptions)
				fmt.Println(asciiString)
			}

			//fmt.Println(hit.Fields["preview"].(string))
			fmt.Println(strings.ReplaceAll(hit.Fields["content"].(string), "\n", " "))

		}

	default:
	}

}
