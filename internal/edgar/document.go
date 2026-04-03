package edgar

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type FilingDocument struct {
	Ticker          string
	CompanyName     string
	CIK             string
	Form            string
	FilingDate      string
	AccessionNumber string
	URL             string
	RawHTML         string
	CleanText       string
}

func (c *Client) GetFilingDocument(ctx context.Context, cik string, filing Filing) (*FilingDocument, error) {
	accNoDash := strings.ReplaceAll(filing.AccessionNumber, "-", "")
	cikNum := strings.TrimLeft(cik, "0")
	url := fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s/%s/%s",
		cikNum, accNoDash, filing.PrimaryDocument)

	data, err := c.get(ctx, url, 7*24*time.Hour)
	if err != nil {
		// Build the EDGAR viewer URL as fallback
		viewerURL := fmt.Sprintf("https://www.sec.gov/cgi-bin/browse-edgar?action=getcompany&CIK=%s&type=&dateb=&owner=include&count=40", cikNum)
		return nil, fmt.Errorf("filing not available at:\n  %s\n\n  Try browsing: %s", url, viewerURL)
	}

	cleanText := htmlToText(string(data))

	return &FilingDocument{
		CIK:             cik,
		Form:            filing.Form,
		FilingDate:       filing.FilingDate.Format("2006-01-02"),
		AccessionNumber: filing.AccessionNumber,
		URL:             url,
		RawHTML:         string(data),
		CleanText:       cleanText,
	}, nil
}

func (c *Client) ListFilings(ctx context.Context, cik string, formTypes []string, limit int) ([]Filing, error) {
	subs, err := c.GetSubmissions(ctx, cik)
	if err != nil {
		return nil, err
	}

	since := time.Now().AddDate(-3, 0, 0)
	all := FilterFilings(subs, formTypes, since)

	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func htmlToText(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return stripBasicHTML(html)
	}

	doc.Find("script, style, head").Remove()

	text := doc.Find("body").Text()
	if text == "" {
		text = doc.Text()
	}

	return cleanText(text)
}

func stripBasicHTML(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			result.WriteRune(' ')
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return cleanText(result.String())
}

func cleanText(s string) string {
	lines := strings.Split(s, "\n")
	var cleaned []string
	prevEmpty := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if !prevEmpty {
				cleaned = append(cleaned, "")
				prevEmpty = true
			}
			continue
		}
		prevEmpty = false
		cleaned = append(cleaned, line)
	}

	result := strings.Join(cleaned, "\n")
	result = strings.ReplaceAll(result, "\u00a0", " ")
	result = strings.ReplaceAll(result, "&nbsp;", " ")
	result = strings.ReplaceAll(result, "&amp;", "&")
	result = strings.ReplaceAll(result, "&lt;", "<")
	result = strings.ReplaceAll(result, "&gt;", ">")
	return strings.TrimSpace(result)
}
