import pg from "pg";
import { Jumbo } from "./scrapers/jumbo.js";

const { Pool } = pg;

const pool = new Pool({
  connectionString: process.env.DATABASE_URL,
  ssl: process.env.DATABASE_URL?.includes("railway")
    ? { rejectUnauthorized: false }
    : false,
});

/* ============== NORMALIZADOR ============== */
function normalize(str = "") {
  return str
    .toLowerCase()
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "") // quita acentos
    .replace(/ñ/g, "n")
    .replace(/\s+/g, "+")
    .replace(/\++/g, "+")
    .replace(/^\+|\+$/g, "");
}

/* ============== MAIN ============== */
async function run() {
  if (!process.env.DATABASE_URL) {
    console.error("DATABASE_URL no definido");
    process.exit(1);
  }

  const products = await Jumbo("");

  let updated = 0;
  let skipped = 0;
  let comparisons = 0;

  const selectByName = `
    SELECT id, name, image, price
    FROM products
    WHERE source = 'jumbo'
      AND normalized_name LIKE $1
  `;

  const updatePrice = `
    UPDATE products
    SET price = $1,
        list_price = $2,
        updated_at = NOW()
    WHERE id = $3
  `;

  const client = await pool.connect();

  try {
    for (const p of products) {
      const normalized = normalize(p.Name);
      let parts = normalized.split("+");
      let matched = false;

      while (parts.length > 0 && !matched) {
        const search = parts.join("+") + "%";

        const res = await client.query(selectByName, [search]);

        for (const row of res.rows) {
          comparisons++;

          // 🔑 VALIDACIÓN FINAL: IMAGE EXACTA
          if (row.image === p.Image) {
            if (Number(row.price) !== Number(p.Price)) {
              await client.query(updatePrice, [
                p.Price,
                p.ListPrice,
                row.id,
              ]);

              console.log(
                `✅ UPDATE | ${row.name} | ${row.price} → ${p.Price}`
              );
            } else {
              console.log(`ℹ️ SIN CAMBIO | ${row.name}`);
            }

            updated++;
            matched = true;
            break;
          }
        }

        // ❌ no hubo match → quitar última palabra
        parts.pop();
      }

      if (!matched) {
        skipped++;
        console.log(`❌ NO MATCH | ${p.Name}`);
      }
    }
  } finally {
    client.release();
  }

  console.log("======== RESUMEN ========");
  console.log("Actualizados:", updated);
  console.log("Sin match:", skipped);
  console.log("Comparaciones image:", comparisons);

  await pool.end();
}

run().catch((e) => {
  console.error(e);
  process.exit(1);
});
