package main

import (
	"context"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

type Product struct {
	Name string
	URL  string
}

type Check struct {
	url      string
	products map[string]Product
}

func NewCheck(url string) *Check {
	return &Check{
		url:      url,
		products: make(map[string]Product),
	}
}

func (c *Check) Do() ([]Product, error) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	var res string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(c.url),
		chromedp.WaitReady(`.info-container-wrapper  .name `),
		chromedp.InnerHTML(`.category--products-wrapper`, &res, chromedp.ByQueryAll),
	); err != nil {
		return nil, err
	}
	//	fmt.Println(res)

	products := make([]Product, 0)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(res))
	if err != nil {
		return nil, err
	}
	doc.Find(".product-card:not(.unavailable)").Each(func(i int, s *goquery.Selection) {
		name := s.Find(".name").Text()
		url, ok := s.Find("a.image").Attr("href")
		if !ok {
			url = "not found"
		}
		// fmt.Printf("%s - %s\n", name, url)
		products = append(products, Product{
			Name: name,
			URL:  url,
		})

	})

	return products, nil
}
