package app

import (
	"context"
	_ "embed"
	"encoding/xml"
	"github.com/jackc/pgx/v5"
	"strings"
)

//go:embed social-site.html
var socialSite []byte

// Scrape only our bundled XHTML fixture. No arbitrary URL fetching or social account access.
func ScrapeSocial() ([]Post, error) {
	var site struct {
		Articles []struct {
			ID         string   `xml:"id,attr"`
			Period     string   `xml:"data-period,attr"`
			Reach      int      `xml:"data-reach,attr"`
			Engagement int      `xml:"data-engagement,attr"`
			Caption    string   `xml:"p"`
			Comments   []string `xml:"ul>li"`
			Hashtags   []string `xml:"tags>tag"`
		} `xml:"body>main>article"`
	}
	if e := xml.Unmarshal(socialSite, &site); e != nil {
		return nil, e
	}
	posts := []Post{}
	for _, p := range site.Articles {
		posts = append(posts, Post{ID: p.ID, Period: p.Period, Reach: p.Reach, Engagement: p.Engagement, Caption: p.Caption, Comments: p.Comments, Hashtags: p.Hashtags})
	}
	return posts, nil
}

type Post struct {
	ID         string   `json:"id"`
	Caption    string   `json:"caption"`
	Comments   []string `json:"comments"`
	Hashtags   []string `json:"hashtags"`
	Period     string   `json:"period"`
	Reach      int      `json:"reach"`
	Engagement int      `json:"engagement"`
}

func (a *App) importSocial(ctx context.Context, tx pgx.Tx, posts []Post) (int, M, error) {
	if len(posts) > 1000 {
		return 400, M{"error": "At most 1000 synthetic posts"}, nil
	}
	products, e := Rows(ctx, tx, "SELECT id,name FROM products")
	if e != nil {
		return 0, nil, e
	}
	n := int64(0)
	for _, p := range posts {
		if p.ID == "" || p.Reach < 0 || p.Engagement < 0 || (p.Period != "baseline" && p.Period != "current") {
			return 400, M{"error": "Post requires ID, baseline/current period, nonnegative metrics"}, nil
		}
		var product any
		content := p.Caption + " " + strings.Join(p.Comments, " ") + " " + strings.Join(p.Hashtags, " ")
		caption := strings.ToLower(content)
		for _, v := range products {
			words := strings.Fields(strings.ToLower(v["name"].(string)))
			if strings.Contains(caption, strings.ToLower(v["id"].(string))) || strings.Contains(caption, words[len(words)-1]) {
				product = v["id"]
				break
			}
		}
		r, e := tx.Exec(ctx, "INSERT INTO social VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING", p.ID, product, p.Period, p.Reach, p.Engagement, content)
		if e != nil {
			return 0, nil, e
		}
		n += r.RowsAffected()
	}
	return 201, M{"imported": n, "source": "Synthetic social-site fixture · keyword matching"}, nil
}
func (a *App) socialSummary(ctx context.Context) M {
	return M{"source": "Synthetic social-site import · keyword matching; reach is fixture impressions, engagement is fixture interactions", "products": a.Query(ctx, `SELECT p.id,p.name,count(s.id) FILTER(WHERE period='baseline')::int baseline,count(s.id) FILTER(WHERE period='current')::int current,coalesce(sum(reach),0)::int reach,coalesce(sum(engagement),0)::int engagement,(count(s.id) FILTER(WHERE period='current')>=3 AND count(s.id) FILTER(WHERE period='current')>=greatest(1,count(s.id) FILTER(WHERE period='baseline'))*2) trending FROM products p LEFT JOIN social s ON s.product=p.id GROUP BY p.id ORDER BY p.id`), "noise": a.Query(ctx, "SELECT count(*) unmatched FROM social WHERE product IS NULL")}
}
