package main

import (
	"database/sql"
	"log"
	"os"
	"strings"
	"unicode"

	_ "github.com/lib/pq"
	"golang.org/x/text/unicode/norm"

	"ratoneando/scrapers"
)

/* ================= NORMALIZADOR ================= */

func normalize(s string) string {
	s = strings.ToLower(s)
	t := norm.NFD.String(s)

	var b strings.Builder
	for _, r := range t {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if r == 'ñ' {
			r = 'n'
		}
		if unicode.IsSpace(r) {
			r = '+'
		}
		b.WriteRune(r)
	}

	out := strings.Trim(b.String(), "+")
	for strings.Contains(out, "++") {
		out = strings.ReplaceAll(out, "++", "+")
	}
	return out
}

/* ================= MAIN ================= */

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("❌ DATABASE_URL no definido")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	products, err := scrapers.Jumbo("")
	if err != nil {
		log.Fatal(err)
	}

	selectByName := `
		SELECT id, name, image, price
		FROM products
		WHERE source = 'jumbo'
		  AND normalized_name LIKE $1
	`

	updatePrice := `
		UPDATE products
		SET price = $1,
		    list_price = $2,
		    updated_at = NOW()
		WHERE id = $3
	`

	for _, p := range products {
		normalized := normalize(p.Name)
		words := strings.Split(normalized, "+")
		found := false

		for len(words) > 0 && !found {
			search := strings.Join(words, "+") + "%"

			rows, err := db.Query(selectByName, search)
			if err != nil {
				log.Println("Query error:", err)
				break
			}

			for rows.Next() {
				var (
					id       string
					dbName   string
					dbImage  string
					oldPrice float64
				)

				if err := rows.Scan(&id, &dbName, &dbImage, &oldPrice); err != nil {
					continue
				}

				// 🔑 VALIDACIÓN FINAL: IMAGE EXACTA
				if dbImage == p.Image {
					if oldPrice != p.Price {
						_, err := db.Exec(updatePrice, p.Price, p.ListPrice, id)
						if err == nil {
							log.Printf("✅ UPDATE [%s] %.2f → %.2f\n", dbName, oldPrice, p.Price)
						}
					} else {
						log.Printf("ℹ️ SIN CAMBIO [%s]\n", dbName)
					}
					found = true
					break
				}
			}

			rows.Close()

			// ❌ no encontró → quitar última palabra
			words = words[:len(words)-1]
		}

		if !found {
			log.Printf("❌ NO MATCH [%s]\n", p.Name)
		}
	}

	log.Println("🎯 Proceso finalizado")
}
