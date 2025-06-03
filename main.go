package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"gopkg.in/gomail.v2"
)

const (
	URL = "https://www.hofweb.nl/groente-aardappels/2e-klas-groentes"
)

var (
	mailHost     = flag.String("mail-host", "smtp.example.com", "mail host")
	mailPort     = flag.Int("mail-port", 465, "mail port")
	mailUser     = flag.String("mail-user", "user@example.com", "login user")
	mailPassword = flag.String("mail-password", "secret", "login password")
	mailTo       = flag.String("mail-to", "to@example.com", "to address")
	mailCC       = flag.String("mail-cc", "cc@example.com", "cc address")
	mailFrom     = flag.String("mail-from", "from@example.com", "from address")
)

type MailConfiguration struct {
	Host     string
	Port     int
	User     string
	Password string
	To       string
	CC       string
	From     string
}

func main() {
	flag.Parse()
	mailConf := MailConfiguration{
		Host:     *mailHost,
		Port:     *mailPort,
		User:     *mailUser,
		Password: *mailPassword,
		To:       *mailTo,
		CC:       *mailCC,
		From:     *mailFrom,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	logger.Info("hofweb checker started")

	ticker := time.NewTicker(5 * time.Minute)
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	existingProds := make([]string, 0)
	for {
		select {
		case <-ticker.C:
			logger.Info("checking page...")
			products, err := Check()
			if err != nil {
				logger.Error("could not get products", "error", err)
				continue
			}
			currentProducts := make([]string, 0)
			newProducts := make([]Product, 0)
			for _, p := range products {
				currentProducts = append(currentProducts, p.URL)
				if !slices.Contains(existingProds, p.URL) {
					newProducts = append(newProducts, p)
				}
			}
			logger.Info("fetched products", "total", len(currentProducts), "new", len(newProducts))

			if len(existingProds) != 0 && len(newProducts) > 0 {
				// if len(newProducts) > 0 {
				if err := Notify(mailConf, newProducts); err != nil {
					logger.Error("could not notify of new products", "error", err)
					continue
				}
				logger.Info("notification sent")
			}
			existingProds = currentProducts

		case <-c:
			logger.Info("exiting...")
			goto EXIT
		}
	}

EXIT:
	ticker.Stop()
	logger.Info("done")
}

type Product struct {
	Name string
	URL  string
}

func Check() ([]Product, error) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	var res string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(URL),
		chromedp.WaitReady(`.info-container-wrapper  .name `),
		chromedp.InnerHTML(`.category--products-wrapper`, &res, chromedp.ByQueryAll),
	); err != nil {
		return nil, err
	}

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
		products = append(products, Product{
			Name: name,
			URL:  url,
		})

	})

	return products, nil
}

func Notify(conf MailConfiguration, prods []Product) error {
	list := make([]string, 0, len(prods))
	for _, p := range prods {
		list = append(list, fmt.Sprintf("<li><a href=\"https://www.hofweb.nl%s\">%s</a></li>", p.URL, p.Name))
	}
	html := fmt.Sprintf(`<html><body><h1>Nieuwe 2e klas groentes bij Hofweb</h1>
<p>Alle producten: <a href="%s">hofweb.nl/groente-aardappels</a></p>
<ul>
%s
</ul></body></html>`, URL, strings.Join(list, "\n"))

	m := gomail.NewMessage()
	m.SetHeader("From", conf.From)
	m.SetHeader("To", conf.To)
	m.SetHeader("Cc", conf.CC)
	m.SetHeader("Subject", "Nieuwe 2e klas groentes beschikbaar op hofweb.nl")
	m.SetBody("text/html", html)

	d := gomail.NewDialer(conf.Host, conf.Port, conf.User, conf.Password)

	if err := d.DialAndSend(m); err != nil {
		return err
	}

	return nil
}
