package execute

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"time"

	"github.com/gocolly/colly/v2"

	mapset "github.com/deckarep/golang-set/v2"
)

const persistFileName = "persist.json"

type page struct {
	Items []item `json:"items"`
}

type item struct {
	TextHash string `json:"texthash"`
	Text     string `json:"-"`
	PageURL  string `json:"-"`
}

func hasher(toHash string) string {
	h := sha256.New()
	h.Write([]byte(toHash))
	return hex.EncodeToString(h.Sum(nil))
}

func initializeCollector() *colly.Collector {
	c := colly.NewCollector(
		// Visit only domains: mygarage.ro
		colly.AllowedDomains("www.mygarage.ro"),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/114.0"),
	)

	// Set max Parallelism and introduce a Random Delay
	c.Limit(&colly.LimitRule{
		Parallelism: 2,
		RandomDelay: 5 * time.Second,
	})

	// Before making a request print "Visiting ..."
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL.String())

	})

	return c
}

func Execute(inheritedPersist map[string]page) map[string]page {
	posts := []item{}

	// configuration ------------------------------------------------------------------------------------------------------------------------------------------------------------------------

	collector := initializeCollector()

	collector.OnHTML(".ppost", func(e *colly.HTMLElement) {
		temp := item{}
		temp.Text = e.ChildText("[id^=post_message]")
		temp.TextHash = hasher(temp.Text)
		temp.PageURL = e.Request.URL.String()
		posts = append(posts, temp)
	})

	// Clicks next button
	collector.OnHTML("[rel=next]", func(h *colly.HTMLElement) {
		t := h.Attr("href")
		collector.Visit(t)
	})

	// read persist file to see which from which page to start
	var startPage string

	previousRunPersistStruct := map[string]page{}

	if inheritedPersist == nil {
		previousRunPersist, err := os.ReadFile(persistFileName)
		if err != nil && !os.IsNotExist(err) {
			fmt.Println(err)
			return nil
		}

		if os.IsNotExist(err) && len(os.Args) < 2 {
			fmt.Println("Persist file not existing and no start page provided as argument in CLI. Quitting")
			return nil
		}

		if len(os.Args) > 1 {
			startPage = os.Args[1]
		} else {

			err = json.Unmarshal(previousRunPersist, &previousRunPersistStruct)
			if err != nil {
				fmt.Println(err)
				return nil
			}

			for firstPage := range previousRunPersistStruct {
				startPage = firstPage
				break
			}
		}
	} else {
		previousRunPersistStruct = inheritedPersist
	}

	// start walk ------------------------------------------------------------------------------------------------------------------------------------------------------------------------

	startWalkURL := "https://www.mygarage.ro/componente/110958-cele-mai-bune-oferte-ale-zilei-cititi-regula-din-primul-post-inainte-sa-postati-" + startPage + ".html"
	err := collector.Visit(startWalkURL)
	if err != nil {
		fmt.Println(err)
	}

	collector.Wait()

	// persist & notify ------------------------------------------------------------------------------------------------------------------------------------------------------------------------

	pageNumRegex, _ := regexp.Compile("([0-9]+).html$")

	pages := map[int]page{}

	var lastPageNum int

	for _, post := range posts {
		// grab only number from end
		pageNumMatch := pageNumRegex.FindSubmatch([]byte(post.PageURL))
		pageNum, err := strconv.Atoi(string(pageNumMatch[1]))
		lastPageNum = pageNum
		if err != nil {
			fmt.Println(err)
			return nil
		}

		if entry, ok := pages[pageNum]; ok {
			entry.Items = append(entry.Items, post)

			pages[pageNum] = entry

		} else {
			pages[pageNum] = page{Items: []item{post}}
		}
	}

	lastTwoPages := map[int]page{
		lastPageNum:     pages[lastPageNum],
		lastPageNum - 1: pages[lastPageNum-1],
	}

	// these only have hashes
	lastTwoPagesOnlyHash := map[string]page{}
	// transform current run persist to match new persist structure (leave only hash)
	for key, val := range lastTwoPages {
		itemsSlice := []item{}
		for _, comment := range val.Items {
			itemsSlice = append(itemsSlice, item{
				TextHash: comment.TextHash,
			})
		}
		lastTwoPagesOnlyHash[strconv.Itoa(key)] = page{
			Items: itemsSlice,
		}
	}

	// rewrite persist and notify only if structs differ
	if !reflect.DeepEqual(lastTwoPagesOnlyHash, previousRunPersistStruct) {
		// find the entry that is not in persist

		// flatten all items from both current run and last run (previous only has hash)
		flattenedCurrentOnlyHash := mapset.NewSet[item]()
		flattenedPreviousOnlyHash := mapset.NewSet[item]()
		for _, commentList := range lastTwoPagesOnlyHash {
			flattenedCurrentOnlyHash = mapset.NewSet[item](commentList.Items...)
		}
		for _, commentList := range previousRunPersistStruct {
			flattenedPreviousOnlyHash = mapset.NewSet[item](commentList.Items...)
		}

		diff := flattenedCurrentOnlyHash.Difference(flattenedPreviousOnlyHash)

		// search which are the diff messages that match the hash
		for _, commentList := range lastTwoPages {
			for _, comment := range commentList.Items {
				if diff.Contains(item{TextHash: comment.TextHash}) {
					fmt.Println(comment.Text)
				}
			}
		}

		// write to file
		marshaledPages, _ := json.Marshal(lastTwoPages)
		err := os.WriteFile(persistFileName, marshaledPages, 0644)
		if err != nil {
			fmt.Println(err)
			return nil
		}
	}

	return lastTwoPagesOnlyHash
}
