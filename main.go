package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/codescot/go-common/fileio"
	"github.com/codescot/go-common/httputil"
)

const (
	listTypeBasic = "basic"
	listTypeHost  = "host"
)

type blacklist struct {
	URL       string `json:"url"`
	SkipLines int    `json:"skipLines"`
	Type      string `json:"type"`
}

type acl struct {
	Identifier string      `json:"identifier"`
	Blacklists []blacklist `json:"blacklists"`
}

func fetchBlacklist(wg *sync.WaitGroup, cache *StringCache, source blacklist) {
	defer wg.Done()
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()

	req := httputil.HTTP{
		TargetURL: source.URL,
		Method:    http.MethodGet,
	}

	data, err := req.String()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	domains := strings.Split(data, "\n")
	for i := source.SkipLines; i < len(domains); i++ {
		domain := domains[i]

		if len(domain) <= 0 {
			continue
		}

		if domain[0] == '#' {
			continue
		}

		switch source.Type {
		case listTypeBasic:
			cache.Add(domain)
			break
		case listTypeHost:
			notabs := strings.ReplaceAll(domain, "\t", " ")
			tokens := strings.Split(notabs, " ")
			value := func(t []string) string {
				for j := 1; j < len(t); j++ {
					v := t[j]
					if len(v) <= 1 {
						continue
					}

					if v[0] == '#' {
						continue
					}

					return v
				}

				return "*"
			}(tokens)

			if value == "*" {
				continue
			}

			cache.Add(value)
			break
		}
	}

	fmt.Printf("Added: %s\n", source.URL)
}

const fileSuffix = ".txt"
const hashSuffix = ".md5"

func newMD5Hash(target string) {
	file, err := os.Open(target + fileSuffix)
	if err != nil {
		log.Fatal(err)
	}

	data := md5.New()
	_, err = io.Copy(data, file)
	if err != nil {
		log.Fatal(err)
	}

	file.Close()

	fileHash := data.Sum(nil)
	fileHashString := fmt.Sprintf("%x", fileHash)

	fmt.Printf("\nmd5: %s\n", fileHashString)

	md5filename := target + hashSuffix

	os.Remove(md5filename)
	md5file, err := os.OpenFile(md5filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	md5file.WriteString(fileHashString)
	md5file.Close()
}

func main() {
	var filename string
	flag.StringVar(&filename, "target", "sources/default.json", "-target=path/to/sources.json")
	flag.Parse()

	fmt.Printf("using %s\n\n", filename)
	var lists acl
	fileio.ReadJSON(filename, &lists)

	cache := Strings()

	var wg sync.WaitGroup
	for _, source := range lists.Blacklists {
		wg.Add(1)
		go fetchBlacklist(&wg, cache, source)
	}

	wg.Wait()

	cache.Sort()

	dedupedCache := Strings()
	for _, domain := range cache.data {
		canonicalDomain := domain + "."
		if !dedupedCache.Contains(canonicalDomain) {
			dedupedCache.Add(canonicalDomain)
		}
	}

	fmt.Printf("aggregated and sorted %d domains\n", dedupedCache.Size)

	blacklistFilename := lists.Identifier + fileSuffix
	os.Remove(blacklistFilename)
	file, err := os.OpenFile(blacklistFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	file.WriteString(dedupedCache.All())
	file.Close()

	newMD5Hash(lists.Identifier)
}
