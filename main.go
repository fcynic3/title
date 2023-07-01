package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"golang.org/x/net/html"
)

func main() {
	urlsFile := flag.String("w", "", "File containing URLs")
	proxyURL := flag.String("p", "", "Proxy URL")
	flag.Parse()

	if *urlsFile == "" {
		fmt.Println("Please provide a file containing URLs with -w option.")
		return
	}

	urls, err := readURLs(*urlsFile)
	if err != nil {
		fmt.Printf("Error reading URLs from file: %v\n", err)
		return
	}

	proxyFunc := getProxyFunc(*proxyURL)

	var wg sync.WaitGroup
	for i := 0; i < len(urls); i += 20 {
		end := i + 20
		if end > len(urls) {
			end = len(urls)
		}

		batch := urls[i:end]

		wg.Add(1)
		go sendBatchRequests(batch, proxyFunc, &wg)
	}

	wg.Wait()
}

func readURLs(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		urls = append(urls, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return urls, nil
}

func getProxyFunc(proxyURL string) func(*http.Request) (*url.URL, error) {
	if proxyURL == "" {
		return http.ProxyFromEnvironment
	}

	proxy, err := url.Parse(proxyURL)
	if err != nil {
		fmt.Printf("Invalid proxy URL: %v\n", err)
		return nil
	}

	return http.ProxyURL(proxy)
}

func sendBatchRequests(urls []string, proxyFunc func(*http.Request) (*url.URL, error), wg *sync.WaitGroup) {
	defer wg.Done()

	transport := &http.Transport{
		Proxy:           proxyFunc,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{
		Transport: transport,
	}

	for _, u := range urls {
		wg.Add(1)
		go sendRequest(u, client, &wg)
	}
}

func sendRequest(url string, client *http.Client, wg *sync.WaitGroup) {
	defer wg.Done()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("Error creating request for URL %s: %v\n", url, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending request for URL %s: %v\n", url, err)
		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body for URL %s: %v\n", url, err)
		return
	}

	title := extractTitle(body)
	fmt.Printf("%s (%s)\n", title, url)
}

func extractTitle(body []byte) string {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return ""
	}

	var getTitle func(*html.Node) string
	getTitle = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
			return n.FirstChild.Data
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			title := getTitle(c)
			if title != "" {
				return title
			}
		}

		return ""
	}

	return getTitle(doc)
}
