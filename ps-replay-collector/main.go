package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
)

const replayURL = "https://replay.pokemonshowdown.com"
const replaySearchURL = "https://replay.pokemonshowdown.com/search?output=html&"

func main() {
	args := os.Args
	if len(args) < 4 || len(args) > 5 {
		fmt.Println("go run main.go format limit duration url")
		return
	}

	format := args[1]

	var urls []string
	var err error
	if len(args) == 5 && args[4] != "" {
		urls, err = GetURLsFromForumsPage(args[4], format)
		if err != nil {
			fmt.Println(err)
			return
		}
	} else {
		urls, err = GetURLSFromReplaySearch(format, args[2], args[3])
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	for _, url := range urls {
		fmt.Println(url)
	}
}

func GetURLSFromReplaySearch(format, limit, duration string) ([]string, error) {
	l, err := strconv.Atoi(limit)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse limit: "+limit)
	}

	d, err := time.ParseDuration(duration)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse duration: "+duration)
	}

	return getURLSFromReplaySearch(format, l, time.Now().Add(-d))
}

func getURLSFromReplaySearch(format string, limit int,
	date time.Time) ([]string, error) {

	dateReached := false
	var us []string
	var err error
	urls := make([]string, limit)
	i := 0
	j := 0
	for i <= limit/50 && !dateReached {
		us, dateReached, err = getURLSFromReplayPage(i+1, format, date)
		if err != nil {
			return nil, err
		}
		lenToAdd := len(us)
		if lenToAdd > limit-50*i {
			lenToAdd = limit - 50*i
		}
		for x := 0; x < lenToAdd; x++ {
			urls[i*50+x] = us[x]
			j++
		}
		i++
	}

	return urls[:j], nil
}

func getURLSFromReplayPage(page int, format string, date time.Time) ([]string, bool, error) {
	resp, err := http.Get(replaySearchURL + "page=" + strconv.Itoa(page) + "&format=" + format)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, false, err
	}

	sel := doc.Find("a")
	urls := make([]string, sel.Length())
	j := 0
	sel.Each(func(_ int, s *goquery.Selection) {
		ref, _ := s.Attr("href")
		urls[j] = replayURL + ref
		j++
	})

	return getReplaysBeforeDate(urls, date)
}

func getReplaysBeforeDate(urls []string, date time.Time) ([]string, bool, error) {
	high := len(urls) - 1
	// Too lazy to do dichotomy search
	for high > 0 {
		then, err := getReplayDate(urls[high])
		if err != nil {
			return nil, false, err
		}

		if date.Before(*then) {
			return urls[:high+1], high != len(urls)-1, nil
		}

		high--
	}

	return urls[:high], true, nil
}

func getReplayDate(url string) (*time.Time, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	sel := doc.Find(".uploaddate")
	ts, _ := sel.First().Attr("data-timestamp")
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return nil, err
	}

	res := time.Unix(tsInt, 0)
	return &res, nil
}

func GetURLsFromForumsPage(url, format string) ([]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	sel := doc.Find("a")
	urls := make([]string, sel.Length())
	j := 0
	sel.Each(func(_ int, s *goquery.Selection) {
		ref, _ := s.Attr("href")
		if strings.HasPrefix(ref, "replay") {
			ref = strings.Replace(ref, "replay", "https://replay", 1)
		}
		ref = strings.Replace(ref, "http://", "https://", 1)
		if strings.HasPrefix(ref, "https://replay.pokemonshowdown.com/"+format) ||
			strings.HasPrefix(ref, "https://replay.pokemonshowdown.com/smogtours-"+format) {
			urls[j] = ref
			j++
		}
	})

	return urls[:j], nil
}
