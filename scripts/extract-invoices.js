#!/usr/bin/env node
/**
 * Script para extraer lista de compras de facturas en imágenes
 * Genera un CSV unificado con todos los items comprados
 */

const fs = require('fs');
const path = require('path');

const INVOICES_DIR = path.join(__dirname, '../test-workspace/compras');
const OUTPUT_CSV = path.join(__dirname, '../test-workspace/compras/compras-unificado.csv');
const LLM_ENDPOINT = process.env.LLM_ENDPOINT || 'http://localhost:11434';
const VISION_MODEL = process.env.VISION_MODEL || 'llama3.2-vision:latest'; // Modelo de visión de Ollama

/**
 * Convierte imagen a base64
 */
function imageToBase64(imagePath) {
  const imageBuffer = fs.readFileSync(imagePath);
  return imageBuffer.toString('base64');
}

/**
 * Detecta el tipo MIME de la imagen
 */
function getImageMimeType(imagePath) {
  const ext = path.extname(imagePath).toLowerCase();
  const mimeTypes = {
    '.jpg': 'image/jpeg',
    '.jpeg': 'image/jpeg',
    '.png': 'image/png',
    '.gif': 'image/gif',
    '.webp': 'image/webp'
  };
  return mimeTypes[ext] || 'image/jpeg';
}

/**
 * Extrae texto de una imagen usando Ollama con modelo de visión
 */
async function extractTextFromImageWithVision(imagePath) {
  console.log(`  📸 Procesando: ${path.basename(imagePath)}`);
  
  try {
    const base64Image = imageToBase64(imagePath);
    const mimeType = getImageMimeType(imagePath);
    
    const prompt = `Analiza esta imagen de una factura o recibo de compra. Extrae TODA la información de los productos comprados.

Para cada producto, identifica:
- Nombre/descripción del producto
- Cantidad
- Precio unitario (si está disponible)
- Precio total (si está disponible)
- Categoría o tipo de producto (si es evidente)

Responde SOLO con un JSON válido en este formato:
{
  "fecha": "fecha de la factura si está visible",
  "tienda": "nombre del establecimiento",
  "total": "total de la factura si está visible",
  "productos": [
    {
      "nombre": "nombre del producto",
      "cantidad": "cantidad (número o texto)",
      "precio_unitario": "precio unitario si está disponible",
      "precio_total": "precio total del item",
      "categoria": "categoría si es evidente"
    }
  ]
}

Si no puedes identificar algún campo, usa "N/A" o déjalo vacío.`;

    // Ollama vision API - intentar primero con /api/chat (más moderno)
    let response;
    try {
      response = await fetch(`${LLM_ENDPOINT}/api/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          model: VISION_MODEL,
          messages: [
            {
              role: 'user',
              content: prompt,
              images: [base64Image]
            }
          ],
          stream: false,
          options: {
            temperature: 0.1,
            num_predict: 3000
          }
        })
      });
    } catch (error) {
      // Fallback a /api/generate si /api/chat no funciona
      console.warn(`  ⚠️  Intentando con /api/generate como fallback...`);
      response = await fetch(`${LLM_ENDPOINT}/api/generate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          model: VISION_MODEL,
          prompt: prompt,
          images: [base64Image],
          stream: false,
          options: {
            temperature: 0.1,
            num_predict: 3000
          }
        })
      });
    }

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data = await response.json();
    // /api/chat devuelve message.content, /api/generate devuelve response
    let text = data.message?.content || data.response || '';
    
    if (!text) {
      console.warn(`  ⚠️  Respuesta vacía. Datos recibidos:`, JSON.stringify(data).substring(0, 300));
      throw new Error('Respuesta vacía del modelo');
    }
    
    // Limpiar y extraer JSON de la respuesta
    text = text.trim();
    
    // Intentar extraer JSON si está envuelto en markdown o código
    let jsonText = text;
    
    // Buscar JSON en diferentes formatos
    const jsonMatch = text.match(/```json\s*([\s\S]*?)\s*```/) || 
                      text.match(/```\s*([\s\S]*?)\s*```/) ||
                      text.match(/\{[\s\S]*\}/);
    
    if (jsonMatch) {
      jsonText = jsonMatch[1] || jsonMatch[0];
    }
    
    // Limpiar el texto JSON
    jsonText = jsonText.trim();
    
    // Si empieza con ```, removerlo
    if (jsonText.startsWith('```')) {
      jsonText = jsonText.replace(/^```[a-z]*\n?/, '').replace(/\n?```$/, '');
    }
    
    // Parsear JSON
    try {
      const parsed = JSON.parse(jsonText);
      
      // Validar estructura básica
      if (!parsed.productos || !Array.isArray(parsed.productos)) {
        parsed.productos = [];
      }
      
      return parsed;
    } catch (e) {
      console.warn(`  ⚠️  No se pudo parsear JSON: ${e.message}`);
      console.warn(`  📝 Texto recibido (primeros 500 chars): ${text.substring(0, 500)}`);
      
      // Intentar extraer información básica del texto
      const productos = [];
      const lineas = text.split('\n').filter(l => l.trim());
      
      // Buscar patrones comunes de productos
      for (const linea of lineas) {
        // Buscar líneas que parezcan productos (contienen números que podrían ser precios)
        if (/\d+[.,]\d+/.test(linea) && linea.length > 5) {
          productos.push({
            nombre: linea.substring(0, 50).trim(),
            cantidad: 'N/A',
            precio_unitario: 'N/A',
            precio_total: linea.match(/\d+[.,]\d+/)?.[0] || 'N/A',
            categoria: 'N/A'
          });
        }
      }
      
      return {
        fecha: 'N/A',
        tienda: text.match(/(?:tienda|store|establecimiento)[:\s]+([^\n]+)/i)?.[1]?.trim() || 'N/A',
        total: text.match(/(?:total|suma)[:\s]+([^\n]+)/i)?.[1]?.trim() || 'N/A',
        productos: productos,
        texto_crudo: text.substring(0, 1000)
      };
    }
  } catch (error) {
    console.error(`  ❌ Error procesando imagen: ${error.message}`);
    return {
      fecha: 'N/A',
      tienda: 'N/A',
      total: 'N/A',
      productos: [],
      error: error.message
    };
  }
}

/**
 * Extrae texto usando Tesseract OCR (fallback)
 */
async function extractTextWithOCR(imagePath) {
  const { execSync } = require('child_process');
  
  try {
    // Verificar si tesseract está disponible
    execSync('which tesseract', { stdio: 'ignore' });
    
    console.log(`  🔍 Usando OCR para: ${path.basename(imagePath)}`);
    
    const tempOutput = path.join(__dirname, '../tmp_ocr_output');
    execSync(`tesseract "${imagePath}" "${tempOutput}" -l spa+eng`, { stdio: 'ignore' });
    
    const text = fs.readFileSync(`${tempOutput}.txt`, 'utf-8');
    fs.unlinkSync(`${tempOutput}.txt`);
    
    // Usar LLM para parsear el texto OCR
    return await parseOCRTextWithLLM(text, path.basename(imagePath));
  } catch (error) {
    console.warn(`  ⚠️  Tesseract no disponible: ${error.message}`);
    return null;
  }
}

/**
 * Parsea texto OCR usando LLM
 */
async function parseOCRTextWithLLM(ocrText, filename) {
  const prompt = `Analiza este texto extraído de una factura de compra mediante OCR. Extrae TODA la información de los productos comprados.

Texto extraído:
${ocrText}

Para cada producto, identifica:
- Nombre/descripción del producto
- Cantidad
- Precio unitario (si está disponible)
- Precio total (si está disponible)
- Categoría o tipo de producto (si es evidente)

Responde SOLO con un JSON válido en este formato:
{
  "fecha": "fecha de la factura si está visible",
  "tienda": "nombre del establecimiento",
  "total": "total de la factura si está visible",
  "productos": [
    {
      "nombre": "nombre del producto",
      "cantidad": "cantidad (número o texto)",
      "precio_unitario": "precio unitario si está disponible",
      "precio_total": "precio total del item",
      "categoria": "categoría si es evidente"
    }
  ]
}

Si no puedes identificar algún campo, usa "N/A" o déjalo vacío.`;

  try {
    const response = await fetch(`${LLM_ENDPOINT}/api/generate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        model: process.env.LLM_MODEL || 'llama3.2',
        prompt: prompt,
        stream: false,
        options: {
          temperature: 0.1,
          num_predict: 2000
        }
      })
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data = await response.json();
    let text = data.response || '';
    
    // Limpiar y extraer JSON
    const jsonMatch = text.match(/```json\s*([\s\S]*?)\s*```/) || 
                      text.match(/```\s*([\s\S]*?)\s*```/) ||
                      text.match(/\{[\s\S]*\}/);
    
    if (jsonMatch) {
      text = jsonMatch[1] || jsonMatch[0];
    }
    
    try {
      return JSON.parse(text);
    } catch (e) {
      return {
        fecha: 'N/A',
        tienda: 'N/A',
        total: 'N/A',
        productos: [],
        texto_crudo: text
      };
    }
  } catch (error) {
    console.error(`  ❌ Error parseando OCR: ${error.message}`);
    return {
      fecha: 'N/A',
      tienda: 'N/A',
      total: 'N/A',
      productos: [],
      error: error.message
    };
  }
}

/**
 * Genera CSV unificado
 */
function generateCSV(allItems) {
  const headers = [
    'Factura',
    'Fecha',
    'Tienda',
    'Producto',
    'Cantidad',
    'Precio Unitario',
    'Precio Total',
    'Categoría',
    'Total Factura'
  ];
  
  const rows = [];
  
  for (const item of allItems) {
    rows.push([
      escapeCSV(item.factura),
      escapeCSV(item.fecha),
      escapeCSV(item.tienda),
      escapeCSV(item.producto),
      escapeCSV(item.cantidad),
      escapeCSV(item.precio_unitario),
      escapeCSV(item.precio_total),
      escapeCSV(item.categoria),
      escapeCSV(item.total_factura)
    ]);
  }
  
  const csvContent = [
    headers.join(','),
    ...rows.map(row => row.join(','))
  ].join('\n');
  
  return csvContent;
}

/**
 * Escapa valores para CSV
 */
function escapeCSV(value) {
  if (value === null || value === undefined) {
    return '';
  }
  
  const str = String(value);
  
  // Si contiene comas, comillas o saltos de línea, envolver en comillas
  if (str.includes(',') || str.includes('"') || str.includes('\n')) {
    return `"${str.replace(/"/g, '""')}"`;
  }
  
  return str;
}

/**
 * Función principal
 */
async function main() {
  console.log('🧾 Extrayendo lista de compras de facturas\n');
  console.log(`📁 Directorio: ${INVOICES_DIR}`);
  console.log(`🤖 LLM Endpoint: ${LLM_ENDPOINT}`);
  console.log(`👁️  Modelo de visión: ${VISION_MODEL}\n`);
  
  // Verificar que Ollama está disponible
  try {
    const response = await fetch(`${LLM_ENDPOINT}/api/tags`);
    if (!response.ok) {
      throw new Error('Ollama no está disponible');
    }
    const data = await response.json();
    const modelNames = data.models?.map(m => m.name) || [];
    console.log(`✅ Ollama disponible. Modelos: ${modelNames.join(', ')}\n`);
    
    // Verificar si el modelo de visión está disponible
    if (!modelNames.includes(VISION_MODEL)) {
      console.warn(`⚠️  Modelo ${VISION_MODEL} no encontrado. Modelos disponibles: ${modelNames.join(', ')}`);
      console.warn(`   Instala con: ollama pull ${VISION_MODEL}\n`);
    }
  } catch (error) {
    console.error(`❌ Error conectando con Ollama: ${error.message}`);
    console.error(`   Asegúrate de que Ollama esté corriendo: ollama serve\n`);
    process.exit(1);
  }
  
  // Leer archivos de imágenes
  const files = fs.readdirSync(INVOICES_DIR)
    .filter(f => /\.(jpg|jpeg|png|gif|webp)$/i.test(f))
    .map(f => path.join(INVOICES_DIR, f));
  
  if (files.length === 0) {
    console.error('❌ No se encontraron archivos de imagen en el directorio');
    process.exit(1);
  }
  
  console.log(`📄 Encontradas ${files.length} factura(s)\n`);
  
  const allItems = [];
  
  // Procesar cada factura
  for (let i = 0; i < files.length; i++) {
    const file = files[i];
    const filename = path.basename(file);
    console.log(`[${i + 1}/${files.length}] Procesando: ${filename}`);
    
    let invoiceData = null;
    
    // Intentar primero con visión
    try {
      invoiceData = await extractTextFromImageWithVision(file);
    } catch (error) {
      console.warn(`  ⚠️  Error con visión, intentando OCR...`);
      invoiceData = await extractTextWithOCR(file);
    }
    
    if (!invoiceData || !invoiceData.productos || invoiceData.productos.length === 0) {
      console.warn(`  ⚠️  No se pudieron extraer productos de esta factura\n`);
      continue;
    }
    
    console.log(`  ✅ Extraídos ${invoiceData.productos.length} producto(s)`);
    console.log(`  🏪 Tienda: ${invoiceData.tienda || 'N/A'}`);
    console.log(`  📅 Fecha: ${invoiceData.fecha || 'N/A'}`);
    console.log(`  💰 Total: ${invoiceData.total || 'N/A'}\n`);
    
    // Agregar items a la lista unificada
    for (const producto of invoiceData.productos) {
      allItems.push({
        factura: filename,
        fecha: invoiceData.fecha || 'N/A',
        tienda: invoiceData.tienda || 'N/A',
        producto: producto.nombre || 'N/A',
        cantidad: producto.cantidad || 'N/A',
        precio_unitario: producto.precio_unitario || 'N/A',
        precio_total: producto.precio_total || 'N/A',
        categoria: producto.categoria || 'N/A',
        total_factura: invoiceData.total || 'N/A'
      });
    }
  }
  
  // Generar CSV
  if (allItems.length === 0) {
    console.error('❌ No se pudieron extraer productos de ninguna factura');
    process.exit(1);
  }
  
  console.log(`\n📊 Total de items extraídos: ${allItems.length}`);
  console.log(`💾 Generando CSV...`);
  
  const csvContent = generateCSV(allItems);
  fs.writeFileSync(OUTPUT_CSV, csvContent, 'utf-8');
  
  console.log(`✅ CSV generado exitosamente: ${OUTPUT_CSV}`);
  console.log(`\n📈 Resumen:`);
  console.log(`   - Facturas procesadas: ${files.length}`);
  console.log(`   - Items totales: ${allItems.length}`);
  console.log(`   - Archivo CSV: ${path.basename(OUTPUT_CSV)}`);
}

// Ejecutar
main().catch(error => {
  console.error('❌ Error fatal:', error);
  process.exit(1);
});

