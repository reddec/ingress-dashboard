package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

var (
	errNot200 = errors.New("non-200 code")
	errNoLogo = errors.New("no logo in meta")
)

func detectIconURL(ctx context.Context, url string) string {
	u, err := mainSrcPageIcon(ctx, url)
	if err == nil {
		return u
	}
	log.Println("detect icon from main page", url, ":", err)
	// fallback to classical icon
	faviconURL := url + "/favicon.ico"
	if pingURL(ctx, faviconURL) == nil {
		return faviconURL
	}

	return ""
}

func mainSrcPageIcon(ctx context.Context, pageURL string) (string, error) {
	// try to load main page and check meta headers
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", errNot200
	}
	doc, err := html.Parse(res.Body)
	if err != nil {
		return "", fmt.Errorf("parse HTML: %w", err)
	}
	logoURL := findIcons(doc)
	if logoURL == "" {
		return "", errNoLogo
	}
	if u, err := url.Parse(logoURL); err == nil && !u.IsAbs() && !strings.HasPrefix(logoURL, "/") {
		logoURL = "/" + logoURL
	}

	return logoURL, nil
}

func findIcons(doc *html.Node) string {
	priorityList := []string{"apple-touch-icon", "shortcut icon", "icon", "alternate icon"}

	root := findChild(doc, "html")
	if root == nil {
		return ""
	}

	head := findChild(root, "head")
	if head == nil {
		return ""
	}

	var links = make(map[string]string)
	for child := head.FirstChild; child != nil; child = child.NextSibling {
		if !(child.Type == html.ElementNode && child.Data == "link") {
			var key string
			var value string
			for _, attr := range child.Attr {
				if attr.Key == "rel" {
					key = attr.Val
				} else if attr.Key == "href" {
					value = attr.Val
				}
			}
			if key != "" && value != "" {
				links[key] = value
			}
		}
	}
	for _, name := range priorityList {
		if u, ok := links[name]; ok {
			return u
		}
	}

	return ""
}

func pingURL(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return errNot200
	}

	return nil
}

func findChild(doc *html.Node, name string) *html.Node {
	for child := doc.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == name {
			return child
		}
	}

	return nil
}
