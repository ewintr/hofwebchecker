package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
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
	hofwebURL = "https://www.hofweb.nl/groente-aardappels/2e-klas-groentes"
)

var (
	mailHost     = flag.String("mail-host", "smtp.example.com", "mail host")
	mailPort     = flag.Int("mail-port", 465, "mail port")
	mailUser     = flag.String("mail-user", "user@example.com", "login user")
	mailPassword = flag.String("mail-password", "secret", "login password")
	mailTo       = flag.String("mail-to", "to@example.com", "to address")
	mailCC       = flag.String("mail-cc", "cc@example.com", "cc address")
	mailFrom     = flag.String("mail-from", "from@example.com", "from address")
	haURL        = flag.String("ha-url", "", "Homeassistant URL (e.g. http://homeassistant:8321)")
	haToken      = flag.String("ha-token", "", "Homeassistant long-lived access token")
	haEntity     = flag.String("ha-entity", "sensor.hofweb_checker", "Homeassistant entity ID")
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

type HAConf struct {
	BaseURL string
	Token   string
	Entity  string
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
	haConf := HAConf{
		BaseURL: *haURL,
		Token:   *haToken,
		Entity:  *haEntity,
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
			if err := UpdateHomeAssistant(haConf, "checking", map[string]any{
				"last_check_start": time.Now().Format(time.RFC3339),
				"friendly_name":    "Hofweb checker",
				"icon":             "mdi:web",
			}); err != nil {
				logger.Error("failed to update homeassistant", "error", err)
			}

			products, err := Check()
			if err != nil {
				logger.Error("could not get products", "error", err)
				if err := UpdateHomeAssistant(haConf, "error", map[string]any{
					"last_error":     err.Error(),
					"last_check_end": time.Now().Format(time.RFC3339),
					"friendly_name":  "Hofweb checker",
					"icon":           "mdi:alert",
				}); err != nil {
					logger.Error("failed to update homeassistant", "error", err)
				}
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
				if err := Notify(mailConf, newProducts); err != nil {
					logger.Error("could not notify of new products", "error", err)
					continue
				}
				logger.Info("notification sent")
			}
			existingProds = currentProducts

			if err := UpdateHomeAssistant(haConf, "idle", map[string]any{
				"last_check_end": time.Now().Format(time.RFC3339),
				"friendly_name":  "Hofweb checker",
				"icon":           "mdi:power",
				"count":          len(existingProds),
			}); err != nil {
				logger.Error("failed to update homeassistant", "error", err)
			}

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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cdpctx, cdpcancel := chromedp.NewContext(ctx)
	defer cdpcancel()

	var res string
	if err := chromedp.Run(cdpctx,
		chromedp.Navigate(hofwebURL),
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
</ul></body></html>`, hofwebURL, strings.Join(list, "\n"))

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

func UpdateHomeAssistant(conf HAConf, status string, attributes map[string]any) error {
	payload := struct {
		State      string                 `json:"state"`
		Attributes map[string]interface{} `json:"attributes"`
	}{
		State:      status,
		Attributes: attributes,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/states/%s", conf.BaseURL, conf.Entity)
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+conf.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Home Assistant API returned status %d", resp.StatusCode)
	}

	return nil
}
