package main

import (
	"database/sql"
	"log"
	"os"
	"strings"

	_ "github.com/lib/pq"
	"ratoneando/scrapers"
)

type ChangeLog struct {
	ID        string
	Name      string
	Image     string
	OldPrice  float64
	NewPrice  float64
	MatchedBy string
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL no definido")
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

	// Statements
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
		WHERE LOWER(name) = LOWER($3)
		  AND source = 'jumbo'
		RETURNING id, name, price
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer updateByName.Close()

	var updated, skipped int
	var logs []ChangeLog

	for _, p := range products {
		// 1) Intento por image
		var id, name string
		var oldPrice float64

		err := updateByImage.
			QueryRow(p.Price, p.ListPrice, p.Image).
			Scan(&id, &name, &oldPrice)

		if err == nil {
			if oldPrice != p.Price {
				logs = append(logs, ChangeLog{
					ID:        id,
					Name:      name,
					Image:     p.Image,
					OldPrice:  oldPrice,
					NewPrice:  p.Price,
					MatchedBy: "image",
				})
			}
			updated++
			continue
		}

		// 2) Fallback por name (si image no matcheó)
		if err == sql.ErrNoRows {
			err2 := updateByName.
				QueryRow(p.Price, p.ListPrice, strings.TrimSpace(p.Name)).
				Scan(&id, &name, &oldPrice)

			if err2 == nil {
				if oldPrice != p.Price {
					logs = append(logs, ChangeLog{
						ID:        id,
						Name:      name,
						Image:     p.Image,
						OldPrice:  oldPrice,
						NewPrice:  p.Price,
						MatchedBy: "name",
					})
				}
				updated++
				continue
			}
		}

		skipped++
	}

	// Logs finales
	log.Println("==== RESUMEN ====")
	log.Println("Actualizados:", updated)
	log.Println("Sin match:", skipped)
	log.Println("Cambios de precio:", len(logs))

	for _, l := range logs {
		log.Printf(
			"[%-5s] %s | %.2f → %.2f | %s\n",
			l.MatchedBy,
			l.Name,
			l.OldPrice,
			l.NewPrice,
			l.ID,
		)
	}
}
