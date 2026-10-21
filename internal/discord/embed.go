package discord

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mathh1s/hamazon/internal/catalog"
	"github.com/mathh1s/hamazon/internal/orders"
)

type payload struct {
	Username        string          `json:"username,omitempty"`
	AvatarURL       string          `json:"avatar_url,omitempty"`
	Embeds          []embed         `json:"embeds"`
	AllowedMentions allowedMentions `json:"allowed_mentions"`
}

type allowedMentions struct {
	Parse []string `json:"parse"`
}

type embed struct {
	Title     string      `json:"title"`
	URL       string      `json:"url,omitempty"`
	Color     int         `json:"color"`
	Fields    []field     `json:"fields"`
	Footer    *footer     `json:"footer,omitempty"`
	Timestamp string      `json:"timestamp,omitempty"`
	Thumbnail *embedImage `json:"thumbnail,omitempty"`
}

type field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type footer struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

type embedImage struct {
	URL string `json:"url"`
}

// Status colors follow the Minecraft chat palette (gold, aqua, green, red).
var statusStyle = map[orders.Status]struct {
	label string
	color int
}{
	orders.StatusPending:   {"Pending", 0xFFAA00},
	orders.StatusApproved:  {"Approved", 0x55FFFF},
	orders.StatusFulfilled: {"Fulfilled", 0x55FF55},
	orders.StatusCancelled: {"Cancelled", 0xFF5555},
}

type embedInput struct {
	order      orders.Order
	lastEvent  *orders.Event
	currencies []catalog.Currency
	publicURL  string
	title      string
}

func buildPayload(in embedInput) payload {
	o := in.order
	style := statusStyle[o.Status]

	e := embed{
		Title:     in.title,
		Color:     style.color,
		Timestamp: o.CreatedAt.UTC().Format(time.RFC3339),
		Footer:    &footer{Text: fmt.Sprintf("Hamazon · %d %s", len(o.Lines), plural(len(o.Lines), "item", "items"))},
	}
	unix := strconv.FormatInt(o.CreatedAt.Unix(), 10)
	e.Fields = append(e.Fields,
		field{Name: "Order", Value: "`" + o.Code + "`", Inline: true},
		field{Name: "Player", Value: escape(o.Username), Inline: true},
		field{Name: "Status", Value: style.label, Inline: true},
		field{Name: "Placed", Value: "<t:" + unix + ":f> (<t:" + unix + ":R>)"},
	)

	var lines []string
	for _, l := range o.Lines {
		lines = append(lines, lineText(l, in.currencies))
	}
	for i, chunk := range chunkLines(lines, 1000) {
		name := "Items"
		if i > 0 {
			name = "Items (continued)"
		}
		e.Fields = append(e.Fields, field{Name: name, Value: chunk})
	}

	var totals []string
	for _, c := range in.currencies {
		if amount := o.Totals[c.ID]; amount > 0 {
			totals = append(totals, fmt.Sprintf("**%d** %s", amount, currencyName(c, amount)))
		}
	}
	e.Fields = append(e.Fields, field{Name: "Total", Value: strings.Join(totals, "\n")})

	if o.Note != "" {
		e.Fields = append(e.Fields, field{Name: "Note from player", Value: escape(truncate(o.Note, 1000))})
	}
	if in.lastEvent != nil && in.lastEvent.AdminName != "" {
		value := fmt.Sprintf("%s by %s <t:%d:R>", statusStyle[in.lastEvent.Status].label, escape(in.lastEvent.AdminName), in.lastEvent.CreatedAt.Unix())
		if in.lastEvent.Note != "" {
			value += "\n" + escape(truncate(in.lastEvent.Note, 500))
		}
		e.Fields = append(e.Fields, field{Name: "Last update", Value: value})
	}

	p := payload{Username: "Hamazon", Embeds: []embed{e}, AllowedMentions: allowedMentions{Parse: []string{}}}
	if in.publicURL != "" {
		p.Embeds[0].URL = fmt.Sprintf("%s/admin/orders/%d", in.publicURL, o.ID)
		p.AvatarURL = in.publicURL + "/mc/item/emerald.png"
		if len(o.Lines) > 0 {
			p.Embeds[0].Thumbnail = &embedImage{URL: in.publicURL + "/mc/item/" + o.Lines[0].Texture + ".png"}
		}
	}
	return p
}

// lineText shows the total number of items to hand out first, since that is
// what an admin collects from their chests.
func lineText(l orders.Line, currencies []catalog.Currency) string {
	price := priceText(l.UnitPrice, currencies)
	if l.Count == 1 {
		return fmt.Sprintf("`%d×` **%s** · %s each", l.Quantity, escape(l.Name), price)
	}
	return fmt.Sprintf("`%d×` **%s** (%d × %d) · %s per %d", l.Quantity*l.Count, escape(l.Name), l.Quantity, l.Count, price, l.Count)
}

func priceText(p catalog.Price, currencies []catalog.Currency) string {
	var parts []string
	for _, c := range currencies {
		if amount := p[c.ID]; amount > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", amount, currencyName(c, amount)))
		}
	}
	return strings.Join(parts, " + ")
}

func currencyName(c catalog.Currency, amount int) string {
	if amount == 1 {
		return c.Name
	}
	return c.Name + "s"
}

func chunkLines(lines []string, limit int) []string {
	var out []string
	var cur strings.Builder
	for _, l := range lines {
		if cur.Len() > 0 && cur.Len()+1+len(l) > limit {
			out = append(out, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteByte('\n')
		}
		cur.WriteString(l)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

var markdownEscaper = strings.NewReplacer(
	`\`, `\\`, `*`, `\*`, `_`, `\_`, `~`, `\~`, "`", "\\`", `|`, `\|`, `>`, `\>`, `<`, `\<`, `#`, `\#`, `[`, `\[`, `]`, `\]`, `@`, "@​",
)

// escape keeps player supplied text from turning into markdown, links or pings.
func escape(s string) string { return markdownEscaper.Replace(s) }

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
