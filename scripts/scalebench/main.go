// Command scalebench reads a disposable benchseed instance over IPv4 loopback.
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"sync"
	"time"
)

type sample struct {
	ms    float64
	bytes int64
	ok    bool
	miss  bool
}
type result struct {
	Requests            int
	Errors              int
	CacheMisses         int
	BodyBytes           int64
	Seconds             float64
	P50ms, P95ms, P99ms float64
}

func measure(count, workers int, fetch func(int) sample) result {
	values := make([]sample, count)
	start := make(chan struct{})
	var ready, done sync.WaitGroup
	ready.Add(workers)
	done.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			for i := worker; i < count; i += workers {
				values[i] = fetch(i)
			}
		}()
	}
	ready.Wait()
	t := time.Now()
	close(start)
	done.Wait()
	r := result{Requests: count, Seconds: time.Since(t).Seconds()}
	var times []float64
	for _, v := range values {
		if !v.ok {
			r.Errors++
		} else {
			times = append(times, v.ms)
		}
		if v.miss {
			r.CacheMisses++
		}
		r.BodyBytes += v.bytes
	}
	sort.Float64s(times)
	if len(times) > 0 {
		p := func(frac float64) float64 { return times[int(math.Ceil(float64(len(times))*frac))-1] }
		r.P50ms, r.P95ms, r.P99ms = p(.50), p(.95), p(.99)
	}
	return r
}

func main() {
	port := flag.Int("port", 18081, "Loopback port of a disposable seeded server")
	burst := flag.Int("burst", 10000, "Concurrent simulated readers in each burst")
	flag.Parse()
	if *port < 1 || *port > 65535 {
		panic("invalid port")
	}
	if *burst < 1 || *burst > 10000 {
		panic("burst must be between 1 and 10000")
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", *port)
	tr := &http.Transport{DisableCompression: true, MaxIdleConnsPerHost: 128, MaxIdleConns: 128}
	client := &http.Client{Transport: tr, Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer tr.CloseIdleConnections()
	get := func(path string) sample {
		t := time.Now()
		req, _ := http.NewRequest("GET", base+path, nil)
		req.Header.Set("Accept-Encoding", "gzip")
		resp, err := client.Do(req)
		if err != nil {
			return sample{}
		}
		defer resp.Body.Close()
		n, err := io.Copy(io.Discard, resp.Body)
		return sample{float64(time.Since(t).Microseconds()) / 1000, n, resp.StatusCode == 200 && err == nil && n > 0, resp.Header.Get("X-Cache") == "MISS"}
	}
	r := map[string]result{}
	r["cold_timeline"] = measure(128, 128, func(int) sample { return get("/") })
	resp, err := client.Get(base + "/")
	if err != nil {
		panic(err)
	}
	html, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		panic(err)
	}
	ids := regexp.MustCompile(`/media/([a-zA-Z0-9_-]+)`).FindAllSubmatch(html, -1)
	var images []string
	seen := map[string]bool{}
	for _, id := range ids {
		path := "/media/" + string(id[1])
		if !seen[path] {
			seen[path] = true
			images = append(images, path)
		}
	}
	if len(images) < 5 {
		panic("expected an image-heavy timeline; seed every second post with a photograph")
	}
	r["cold_thumbnails"] = measure(len(images), len(images), func(i int) sample { return get(images[i] + "?w=480") })
	r["mixed_64"] = measure(4096, 64, func(i int) sample {
		switch i % 8 {
		case 0, 1:
			return get("/")
		case 2:
			return get(fmt.Sprintf("/?post=seed-%05d", i%10000))
		case 3:
			return get(fmt.Sprintf("/?q=Group%%20%d", i%127))
		case 4:
			return get("/feed.md")
		case 5:
			return get("/feed.xml")
		case 6:
			return get(images[(i/8)%len(images)] + "?w=480")
		default:
			return get(images[(i/8)%len(images)])
		}
	})
	search := "/api/v1/list_posts?input=" + url.QueryEscape(`{"search":"gardens","lang":"en","limit":20}`)
	r["uncached_search"] = measure(512, 32, func(int) sample { return get(search) })
	cursor := base64.RawURLEncoding.EncodeToString([]byte("1700000000100/seed-00100"))
	deep := "/api/v1/list_posts?input=" + url.QueryEscape(fmt.Sprintf(`{"cursor":%q,"limit":20}`, cursor))
	r["deep_cursor"] = measure(512, 32, func(int) sample { return get(deep) })
	if !get("/").ok {
		panic("timeline warmup failed")
	}
	r["warm_timeline_burst"] = measure(*burst, *burst, func(int) sample { return get("/") })
	// Each visitor reads the HTML and then its first two photographs. Include
	// HTTP negotiation, transfer and client scheduling through the last byte.
	r["two_photo_visits"] = measure(*burst, *burst, func(int) sample {
		start := time.Now()
		page := get("/")
		for _, path := range images[:2] {
			v := get(path + "?w=480")
			page.ok = page.ok && v.ok
			page.bytes += v.bytes
		}
		page.ms = float64(time.Since(start).Microseconds()) / 1000
		return page
	})
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		panic(err)
	}
	for _, v := range r {
		if v.Errors > 0 {
			os.Exit(1)
		}
	}
}
