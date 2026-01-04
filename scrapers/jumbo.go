import logger from "./utils/logger.js"; // Asumiendo que tienes un logger similar

/**
 * Realiza la búsqueda de productos en la API interna de Jumbo (VTEX)
 * @param {string} query - El término de búsqueda
 * @returns {Promise<Array>} - Lista de productos normalizados
 */
export async function searchJumbo(query) {
    const baseUrl = "https://www.jumbo.com.ar";
    const endpoint = `${baseUrl}/api/catalog_system/pub/products/search/?ft=${encodeURIComponent(query)}`;

    try {
        const response = await fetch(endpoint);
        
        if (!response.ok) {
            throw new Error(`Error HTTP: ${response.status}`);
        }

        const data = await response.json();

        // Mapeo y Normalización (Similar al Normalizer + Extractor de tu Go)
        return data.map(product => {
            try {
                // El campo ProductData en VTEX suele venir como un string JSON dentro de un array
                const rawProductData = product.ProductData && product.ProductData[0] 
                    ? JSON.parse(product.ProductData[0]) 
                    : {};

                const item = product.items[0];
                const seller = item.sellers[0].commertialOffer;

                // Estructura final (Schema)
                return {
                    id: product.productId,
                    source: "jumbo",
                    name: product.productName,
                    link: product.link,
                    image: item.images[0].imageUrl,
                    unavailable: !seller.IsAvailable,
                    price: seller.Price,
                    listPrice: seller.ListPrice,
                    unit: rawProductData.MeasurementUnit || "un",
                    unitFactor: rawProductData.UnitMultiplier || 1
                };
            } catch (err) {
                logger.warn(`Error procesando producto ${product.productId}: ${err.message}`);
                return null;
            }
        }).filter(p => p !== null); // Filtramos los que fallaron

    } catch (error) {
        logger.error(`Error en el scraper de Jumbo: ${error.message}`);
        return [];
    }
}

// Ejemplo de uso:
// (async () => {
//    const productos = await searchJumbo("detergente");
//    console.log(productos);
// })();
