package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
)

const (
	defaultBaseURL = "https://s-file-1.ykt.cbern.com.cn"
	defaultReferer = "https://basic.smartedu.cn/"
	defaultUA      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36"
)

// Keep compatibility with Go 1.16 (no predeclared any).
type any = interface{}

type downloadRecord struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Path      string `json:"path"`
	Bytes     int64  `json:"bytes"`
	Succeeded bool   `json:"succeeded"`
	Error     string `json:"error,omitempty"`
}

type runSummary struct {
	CourseID      string           `json:"courseId"`
	OutputDir     string           `json:"outputDir"`
	GeneratedAt   string           `json:"generatedAt"`
	AudioCount    int              `json:"audioCount"`
	AudioSuccess  int              `json:"audioSuccess"`
	PDFCount      int              `json:"pdfCount"`
	PDFSuccess    int              `json:"pdfSuccess"`
	TotalBytes    int64            `json:"totalBytes"`
	FailedCount   int              `json:"failedCount"`
	DownloadItems []downloadRecord `json:"items"`
}

type audioItem struct {
	Title string
	URL   string
}

func main() {
	app := &cli.App{
		Name:    "tch-material-downloader",
		Usage:   "Download all audios and textbook PDF(s) for a SmartEdu material course",
		Version: "v2.6.2",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "mac-id",
				Aliases: []string{"m"},
				Usage:   "Value for x-nd-auth request header",
			},
			&cli.StringFlag{
				Name:    "course-id",
				Aliases: []string{"c"},
				Usage:   "Course content ID, e.g. db7a6bc2-c5d4-4115-88e5-bf6f36cf1914",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output directory (default: ./output/{course-id})",
			},
			&cli.IntFlag{
				Name:  "timeout",
				Value: 45,
				Usage: "HTTP timeout in seconds",
			},
			&cli.StringFlag{
				Name:  "referer",
				Value: defaultReferer,
				Usage: "Referer header for downloading resources",
			},
			&cli.StringFlag{
				Name:  "ua",
				Value: defaultUA,
				Usage: "User-Agent header for downloading resources",
			},
		},
		Action: func(cCtx *cli.Context) error {
			macID := strings.TrimSpace(cCtx.String("mac-id"))
			courseID := strings.TrimSpace(cCtx.String("course-id"))
			if macID == "" {
				v, err := promptRequired("请输入 mac-id (x-nd-auth): ")
				if err != nil {
					return err
				}
				macID = v
			}
			if courseID == "" {
				v, err := promptRequired("请输入课程 ID (course-id): ")
				if err != nil {
					return err
				}
				courseID = v
			}

			if macID == "" {
				return errors.New("mac-id is required")
			}
			if courseID == "" {
				return errors.New("course-id is required")
			}

			output := strings.TrimSpace(cCtx.String("output"))
			if output == "" {
				output = filepath.Join("output", courseID)
			}
			if err := os.MkdirAll(output, 0755); err != nil {
				return fmt.Errorf("create output dir failed: %w", err)
			}

			httpTimeout := time.Duration(cCtx.Int("timeout")) * time.Second
			client := &http.Client{Timeout: httpTimeout}
			dl := &downloader{
				client:  client,
				macID:   macID,
				referer: cCtx.String("referer"),
				ua:      cCtx.String("ua"),
			}

			summary := runSummary{
				CourseID:      courseID,
				OutputDir:     output,
				GeneratedAt:   time.Now().Format(time.RFC3339),
				DownloadItems: make([]downloadRecord, 0),
			}

			ctx := context.Background()
			audioItems, err := dl.loadAudioItems(ctx, courseID)
			if err != nil {
				log.Printf("Load audio list failed, continue with PDF only: %v", err)
				audioItems = nil
			}
			summary.AudioCount = len(audioItems)
			if len(audioItems) == 0 {
				log.Printf("Found 0 audio item(s), will only download PDF if available")
			} else {
				log.Printf("Found %d audio item(s)", len(audioItems))
			}

			for idx, item := range audioItems {
				fileName := fmt.Sprintf("%03d_%s.mp3", idx+1, safeFileName(item.Title, 90))
				filePath := filepath.Join(output, fileName)
				n, err := dl.downloadFile(ctx, item.URL, filePath)
				record := downloadRecord{
					Type:      "audio",
					Title:     item.Title,
					URL:       item.URL,
					Path:      filePath,
					Bytes:     n,
					Succeeded: err == nil,
				}
				if err != nil {
					record.Error = err.Error()
					summary.FailedCount++
					log.Printf("[audio][%d/%d] failed: %s (%v)", idx+1, len(audioItems), item.Title, err)
				} else {
					summary.AudioSuccess++
					summary.TotalBytes += n
					log.Printf("[audio][%d/%d] done: %s (%d bytes)", idx+1, len(audioItems), item.Title, n)
				}
				summary.DownloadItems = append(summary.DownloadItems, record)
			}

			pdfItems, err := dl.loadPDFItems(ctx, courseID)
			if err != nil {
				log.Printf("Load PDF detail failed: %v", err)
			} else {
				summary.PDFCount = len(pdfItems)
				log.Printf("Found %d textbook PDF item(s)", len(pdfItems))
				for idx, item := range pdfItems {
					ext := strings.ToLower(filepath.Ext(item.URL))
					if ext == "" {
						ext = ".pdf"
					}
					fileName := fmt.Sprintf("textbook_%02d_%s%s", idx+1, safeFileName(item.Title, 80), ext)
					filePath := filepath.Join(output, fileName)
					n, dlErr := dl.downloadFile(ctx, item.URL, filePath)
					record := downloadRecord{
						Type:      "pdf",
						Title:     item.Title,
						URL:       item.URL,
						Path:      filePath,
						Bytes:     n,
						Succeeded: dlErr == nil,
					}
					if dlErr != nil {
						record.Error = dlErr.Error()
						summary.FailedCount++
						log.Printf("[pdf][%d/%d] failed: %s (%v)", idx+1, len(pdfItems), item.Title, dlErr)
					} else {
						summary.PDFSuccess++
						summary.TotalBytes += n
						log.Printf("[pdf][%d/%d] done: %s (%d bytes)", idx+1, len(pdfItems), item.Title, n)
					}
					summary.DownloadItems = append(summary.DownloadItems, record)
				}
			}

			manifestPath := filepath.Join(output, "manifest.json")
			if err := writeJSON(manifestPath, summary); err != nil {
				return fmt.Errorf("write manifest failed: %w", err)
			}

			log.Printf("Completed. audio=%d/%d, pdf=%d/%d, failed=%d, bytes=%d", summary.AudioSuccess, summary.AudioCount, summary.PDFSuccess, summary.PDFCount, summary.FailedCount, summary.TotalBytes)
			log.Printf("Manifest saved to %s", manifestPath)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type downloader struct {
	client  *http.Client
	macID   string
	referer string
	ua      string
}

func (d *downloader) loadAudioItems(ctx context.Context, courseID string) ([]audioItem, error) {
	u := fmt.Sprintf("%s/zxx/ndrs/resources/%s/relation_audios.json", defaultBaseURL, url.PathEscape(courseID))
	root, err := d.fetchJSONValue(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("load relation_audios.json failed: %w", err)
	}
	log.Printf("[audio-debug] relation_audios source: %s", u)
	log.Printf("[audio-debug] relation_audios root type: %T", root)

	// Primary parser: match the proven script flow used before Go implementation
	// (top-level item -> ti_items -> href/source mp3).
	primaryItems := collectTopLevelAudioItems(root)
	log.Printf("[audio-debug] extracted top-level audio items: %d", len(primaryItems))
	if len(primaryItems) > 0 {
		result := make([]audioItem, 0, len(primaryItems))
		seen := make(map[string]struct{}, len(primaryItems))
		for idx, item := range primaryItems {
			title := pickTitle(item)
			selectedURL, candidates := pickAudioFromTopLevelItem(item)
			log.Printf("[audio-debug][top-level %d] title=%q candidates=%d selected=%q", idx+1, title, len(candidates), shortURL(selectedURL))
			if len(candidates) > 0 {
				log.Printf("[audio-debug][top-level %d] candidates detail: %s", idx+1, strings.Join(shortURLs(candidates), " | "))
			}
			if selectedURL == "" {
				log.Printf("[audio-debug][top-level %d] skipped: no downloadable URL found", idx+1)
				continue
			}
			if _, ok := seen[selectedURL]; ok {
				log.Printf("[audio-debug][top-level %d] skipped: duplicate selected URL", idx+1)
				continue
			}
			seen[selectedURL] = struct{}{}
			result = append(result, audioItem{Title: title, URL: selectedURL})
		}
		if len(result) > 0 {
			log.Printf("[audio-debug] final downloadable audio items (top-level parser): %d", len(result))
			return result, nil
		}
		log.Printf("[audio-debug] top-level parser found 0 downloadable URLs, fallback to recursive parser")
	}

	itemsRaw := extractAudioRows(root)
	log.Printf("[audio-debug] extracted raw audio rows (fallback): %d", len(itemsRaw))

	result := make([]audioItem, 0, len(itemsRaw))
	seen := make(map[string]struct{}, len(itemsRaw))
	for idx, row := range itemsRaw {
		obj := row
		title := toString(obj["ti_title"])
		if title == "" {
			title = toString(obj["title"])
		}
		if title == "" {
			title = "audio"
		}
		u, candidates := pickAudioURL(obj)
		log.Printf("[audio-debug][row %d] title=%q candidates=%d selected=%q", idx+1, title, len(candidates), shortURL(u))
		if len(candidates) > 0 {
			log.Printf("[audio-debug][row %d] candidates detail: %s", idx+1, strings.Join(shortURLs(candidates), " | "))
		}
		if u == "" {
			log.Printf("[audio-debug][row %d] skipped: no downloadable URL found", idx+1)
			continue
		}
		if _, ok := seen[u]; ok {
			log.Printf("[audio-debug][row %d] skipped: duplicate selected URL", idx+1)
			continue
		}
		seen[u] = struct{}{}
		result = append(result, audioItem{Title: title, URL: u})
	}
	log.Printf("[audio-debug] final downloadable audio items (fallback parser): %d", len(result))
	return result, nil
}

func collectTopLevelAudioItems(root any) []map[string]any {
	out := make([]map[string]any, 0)
	appendItem := func(v any) {
		m, ok := v.(map[string]any)
		if !ok {
			return
		}
		if _, ok := m["ti_items"]; ok {
			out = append(out, m)
		}
	}

	switch x := root.(type) {
	case []any:
		for _, one := range x {
			appendItem(one)
		}
	case map[string]any:
		if items, ok := x["items"].([]any); ok {
			for _, one := range items {
				appendItem(one)
			}
		}
		if items, ok := x["ti_items"].([]any); ok {
			for _, one := range items {
				appendItem(one)
			}
		}
	}

	return out
}

func pickTitle(item map[string]any) string {
	title := toString(item["title"])
	if title != "" {
		return title
	}
	if gt, ok := item["global_title"].(map[string]any); ok {
		title = toString(gt["zh-CN"])
		if title != "" {
			return title
		}
	}
	title = toString(item["ti_title"])
	if title != "" {
		return title
	}
	title = toString(item["id"])
	if title != "" {
		return title
	}
	return "audio"
}

func pickAudioFromTopLevelItem(item map[string]any) (string, []string) {
	tiItems, ok := item["ti_items"].([]any)
	if !ok {
		return "", nil
	}

	var selected map[string]any
	for _, raw := range tiItems {
		ti, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if strings.EqualFold(toString(ti["ti_file_flag"]), "href") && strings.EqualFold(toString(ti["ti_format"]), "mp3") {
			selected = ti
			break
		}
	}
	if selected == nil {
		for _, raw := range tiItems {
			ti, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if strings.EqualFold(toString(ti["ti_file_flag"]), "source") && strings.EqualFold(toString(ti["ti_format"]), "mp3") {
				selected = ti
				break
			}
		}
	}
	if selected == nil {
		return "", nil
	}

	return pickAudioURL(selected)
}

func (d *downloader) loadPDFItems(ctx context.Context, courseID string) ([]audioItem, error) {
	u := fmt.Sprintf("%s/zxx/ndrv2/resources/tch_material/details/%s.json", defaultBaseURL, url.PathEscape(courseID))
	root, err := d.fetchJSONValue(ctx, u)
	if err != nil {
		return nil, err
	}

	urls := findAllResourceURLs(root, isPDFURL)
	urls = dedupeStrings(urls)
	sort.Strings(urls)
	if len(urls) > 1 {
		// Many courses expose multiple aliases for the same PDF; keep one to avoid duplicates.
		urls = urls[:1]
	}

	items := make([]audioItem, 0, len(urls))
	for _, one := range urls {
		title := "textbook"
		items = append(items, audioItem{Title: title, URL: one})
	}
	return items, nil
}

func (d *downloader) fetchJSONValue(ctx context.Context, reqURL string) (any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("request failed: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}
	var root any
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return nil, err
	}
	return root, nil
}

func extractAudioRows(root any) []map[string]any {
	rows := make([]map[string]any, 0)
	seen := make(map[string]struct{})
	appendRow := func(m map[string]any) {
		if m == nil {
			return
		}
		key := fmt.Sprintf("%p", m)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		rows = append(rows, m)
	}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if _, ok := x["ti_storages"]; ok {
				appendRow(x)
			}
			if _, ok := x["storages"]; ok {
				appendRow(x)
			}
			for _, mv := range x {
				walk(mv)
			}
		case []any:
			for _, av := range x {
				walk(av)
			}
		}
	}
	walk(root)

	return rows
}

func (d *downloader) downloadFile(ctx context.Context, reqURL, outputPath string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("x-nd-auth", d.macID)
	req.Header.Set("Referer", d.referer)
	req.Header.Set("User-Agent", d.ua)
	req.Header.Set("Accept", "*/*")

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return 0, fmt.Errorf("request failed: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return 0, err
	}
	tmp := outputPath + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}
	n, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return n, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return n, closeErr
	}
	if err := os.Rename(tmp, outputPath); err != nil {
		_ = os.Remove(tmp)
		return n, err
	}
	return n, nil
}

func pickAudioURL(item map[string]any) (string, []string) {
	candidates := make([]string, 0, 16)
	addURL := func(v any) {
		u := toString(v)
		if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			candidates = append(candidates, u)
		}
	}

	for _, k := range []string{"ti_url", "url", "href", "source", "audio_url", "audioUrl", "file_url", "fileUrl", "src"} {
		addURL(item[k])
	}

	for _, listKey := range []string{"ti_storages", "storages"} {
		arr, ok := item[listKey].([]any)
		if !ok {
			continue
		}
		for _, s := range arr {
			if sStr, ok := s.(string); ok {
				addURL(sStr)
				continue
			}
			obj, ok := s.(map[string]any)
			if !ok {
				continue
			}
			for _, k := range []string{"href", "source", "url", "audio_url", "audioUrl", "file_url", "fileUrl", "src"} {
				addURL(obj[k])
			}
		}
	}

	candidates = dedupeStrings(candidates)
	for _, c := range candidates {
		if strings.Contains(strings.ToLower(c), ".mp3") {
			return c, candidates
		}
	}
	for _, c := range candidates {
		if strings.Contains(c, "r3-ndr-private") {
			return c, candidates
		}
	}
	for _, c := range candidates {
		if strings.Contains(c, "r2-ndr-private") {
			return c, candidates
		}
	}
	for _, c := range candidates {
		if strings.Contains(c, "r1-ndr-private") {
			return c, candidates
		}
	}
	for _, c := range candidates {
		lower := strings.ToLower(c)
		if strings.Contains(lower, "audio") || strings.Contains(lower, ".m4a") || strings.Contains(lower, ".aac") {
			return c, candidates
		}
	}
	if len(candidates) > 0 {
		return candidates[0], candidates
	}
	return "", candidates
}

func shortURLs(urls []string) []string {
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		out = append(out, shortURL(u))
	}
	return out
}

func shortURL(u string) string {
	if u == "" {
		return ""
	}
	if len(u) <= 120 {
		return u
	}
	return u[:117] + "..."
}

func findAllResourceURLs(data any, accept func(string) bool) []string {
	result := make([]string, 0)
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for _, mv := range x {
				walk(mv)
			}
		case []any:
			for _, av := range x {
				walk(av)
			}
		case string:
			if accept(x) {
				result = append(result, x)
			}
		}
	}
	walk(data)
	return result
}

func isPDFURL(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return false
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	ext := strings.ToLower(path.Ext(u.Path))
	return ext == ".pdf"
}

func dedupeStrings(src []string) []string {
	set := make(map[string]struct{}, len(src))
	out := make([]string, 0, len(src))
	for _, s := range src {
		if _, ok := set[s]; ok {
			continue
		}
		set[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func toString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

var invalidChars = regexp.MustCompile(`[\\/:*?"<>|\x00-\x1F]`)

func safeFileName(raw string, maxLen int) string {
	name := strings.TrimSpace(raw)
	name = strings.ReplaceAll(name, "\n", " ")
	name = strings.ReplaceAll(name, "\r", " ")
	name = invalidChars.ReplaceAllString(name, "_")
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		name = "untitled"
	}
	if maxLen > 0 && len(name) > maxLen {
		name = name[:maxLen]
		name = strings.TrimSpace(name)
	}
	return name
}

func writeJSON(filePath string, v any) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func promptRequired(label string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(label)
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		value := strings.TrimSpace(line)
		if value != "" {
			return value, nil
		}
		if errors.Is(err, io.EOF) {
			return "", nil
		}
		fmt.Println("输入不能为空，请重新输入。")
	}
}
