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

	// separar acentos
	t := norm.NFD.String(s)

	var b strings.Builder
	for _, r := range t {
		// quitar acentos
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		// ñ -> n
		if r == 'ñ' {
			r = 'n'
		}
		// espacios -> +
		if unicode.IsSpace(r) {
			r = '+'
		}
		b.WriteRune(r)
	}

	out := b.String()
	out = strings.ReplaceAll(out, "++", "+")
	out = strings.Trim(out, "+")

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

	updateByImage, err := db.Prepare(`
		UPDATE products
		SET price = $1,
		    list_price = $2,
		    updated_at = NOW()
		WHERE image = $3
		  AND source = 'jumbo'
		RETURNING id, name, price
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer updateByImage.Close()

	updateByName, err := db.Prepare(`
		UPDATE products
		SET price = $1,
		    list_price = $2,
		    updated_at = NOW()
		WHERE LOWER(
		  translate(name,
		    'ÁÉÍÓÚÜÑáéíóúüñ',
		    'AEIOUUNaeiouun'
		  )
		) = $3
		  AND source = 'jumbo'
		RETURNING id, name, price
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer updateByName.Close()

	var updated, skipped, priceChanges int

	for _, p := range products {
		var (
			id       string
			name     string
			oldPrice float64
		)

		// ===== 1) MATCH POR IMAGE =====
		err := updateByImage.
			QueryRow(p.Price, p.ListPrice, p.Image).
			Scan(&id, &name, &oldPrice)

		if err == nil {
			if oldPrice != p.Price {
				log.Printf("[IMAGE] %s | %.2f → %.2f\n", name, oldPrice, p.Price)
				priceChanges++
			}
			updated++
			continue
		}

		// ===== 2) FALLBACK POR NAME NORMALIZADO =====
		normName := normalize(p.Name)

		err = updateByName.
			QueryRow(p.Price, p.ListPrice, normName).
			Scan(&id, &name, &oldPrice)

		if err == nil {
			if oldPrice != p.Price {
				log.Printf("[NAME ] %s | %.2f → %.2f\n", name, oldPrice, p.Price)
				priceChanges++
			}
			updated++
			continue
		}

		skipped++
	}

	log.Println("========== RESUMEN ==========")
	log.Println("Actualizados:", updated)
	log.Println("Sin match:", skipped)
	log.Println("Cambios de precio:", priceChanges)
	log.Println("================================")
}
