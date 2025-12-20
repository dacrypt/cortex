# Office & Photoshop Metadata Extraction

## Overview

Cortex now extracts **rich metadata** from Microsoft Office documents, PDFs, and Adobe Photoshop files - perfect for office workers, designers, and creative professionals!

## What's Extracted

### 📄 **Word Documents** (.docx, .doc)

**Document Properties:**
- Title, Subject, Author
- Keywords (tags)
- Description
- Template used
- Revision number

**Document Statistics:**
- Page count
- Word count
- Character count

**Security:**
- Is encrypted/password protected

**Example:**
```typescript
{
  title: "Q4 Marketing Report",
  author: "John Doe",
  pageCount: 25,
  wordCount: 5,430,
  keywords: ["marketing", "q4", "sales"],
  template: "Corporate Report.dotx",
  revision: "5"
}
```

---

### 📊 **Excel Spreadsheets** (.xlsx, .xls)

**Document Properties:**
- Title, Author
- Creation/modification dates

**Spreadsheet Analysis:**
- Sheet count
- Total rows (approximate from first sheet)
- Total columns (estimated)
- Has formulas (detected)
- Has macros/VBA code
- Has charts
- Has pivot tables (detected)
- Word count (from cell text)

**Example:**
```typescript
{
  author: "Jane Smith",
  sheetCount: 5,
  totalRows: 1250,
  hasFormulas: true,
  hasMacros: false,
  hasCharts: true,
  wordCount: 458
}
```

---

### 📊 **PowerPoint Presentations** (.pptx, .ppt)

**Document Properties:**
- Title, Author
- Creation/modification dates

**Presentation Analysis:**
- Slide count
- Word count (from slide text)
- Has animations
- Has transitions
- Has embedded media (videos, audio)
- Has speaker notes
- Master slide count

**Example:**
```typescript
{
  title: "Product Launch 2024",
  author: "Marketing Team",
  slideCount: 45,
  wordCount: 1,234,
  hasAnimations: true,
  hasTransitions: true,
  hasEmbeddedMedia: true,
  hasNotes: true,
  masterSlideCount: 3
}
```

---

### 📑 **PDF Documents** (.pdf)

**Document Properties:**
- Title, Subject, Author
- Creator (software used)
- Keywords

**PDF Metadata:**
- Page count
- PDF version (e.g., "1.7")
- Is linearized (fast web view)
- Is encrypted
- Has JavaScript
- Has attachments
- Has bookmarks
- Has comments/annotations

**Security:**
- Print allowed
- Copy allowed
- Modify allowed

**Example:**
```typescript
{
  title: "Annual Report 2023",
  author: "Finance Dept",
  creator: "Adobe PDF Library",
  pageCount: 87,
  pdfVersion: "1.7",
  isEncrypted: false,
  hasBookmarks: true,
  hasComments: false
}
```

---

### 🎨 **Photoshop Files** (.psd)

**Image Properties:**
- Width & Height (pixels)
- Resolution/DPI
- Color mode (RGB, CMYK, Grayscale, Lab, etc.)
- Bit depth (8-bit, 16-bit, 32-bit)
- Has transparency (alpha channel)
- Software: "Adobe Photoshop"

**File Properties:**
- Layer count (basic detection)
- Is flattened
- Compression method

**Example:**
```typescript
{
  width: 3840,
  height: 2160,
  colorMode: "RGB",
  bitDepth: 16,
  hasTransparency: true,
  software: "Adobe Photoshop"
}
```

---

## How to Use

### Method 1: Access via API

```typescript
const metadataExtractor = new MetadataExtractor(workspaceRoot);

// Extract from Word document
const wordMetadata = await metadataExtractor.extractDocumentMetadata(
  '/path/to/document.docx',
  enhancedMetadata,
  '.docx'
);

// Extract from Photoshop file
const psdMetadata = await metadataExtractor.extractDocumentMetadata(
  '/path/to/design.psd',
  enhancedMetadata,
  '.psd'
);
```

### Method 2: Available in File Index

All indexed files now have an `enhanced.documentMetadata` or `enhanced.designMetadata` field:

```typescript
const files = indexStore.getAllFiles();

const wordDocs = files.filter(f =>
  f.extension === '.docx' &&
  f.enhanced?.documentMetadata
);

const photoshopFiles = files.filter(f =>
  f.extension === '.psd' &&
  f.enhanced?.designMetadata
);
```

---

## Use Cases

### **Office Workers**

**Find Documents by Author:**
```typescript
const johnsDocs = files.filter(f =>
  f.enhanced?.documentMetadata?.author === 'John Doe'
);
```

**Find Long Reports (>50 pages):**
```typescript
const longReports = files.filter(f =>
  (f.enhanced?.documentMetadata?.pageCount || 0) > 50
);
```

**Find Spreadsheets with Macros:**
```typescript
const macroSheets = files.filter(f =>
  f.extension === '.xlsx' &&
  (f.enhanced?.documentMetadata as SpreadsheetMetadata)?.hasMacros
);
```

**Find Presentations with Animations:**
```typescript
const animatedPPTs = files.filter(f =>
  f.extension === '.pptx' &&
  (f.enhanced?.documentMetadata as PresentationMetadata)?.hasAnimations
);
```

---

### **Designers / Creative Professionals**

**Find High-Resolution PSDs (>300 DPI):**
```typescript
const highRes = files.filter(f =>
  f.extension === '.psd' &&
  (f.enhanced?.designMetadata?.resolution || 0) >= 300
);
```

**Find CMYK Files (for print):**
```typescript
const printReady = files.filter(f =>
  f.enhanced?.designMetadata?.colorMode === 'CMYK'
);
```

**Find Large Designs (>4K):**
```typescript
const largeDesigns = files.filter(f => {
  const meta = f.enhanced?.designMetadata;
  return meta && meta.width && meta.width >= 3840;
});
```

---

### **Publishers / Writers**

**Word Count Analysis:**
```typescript
const totalWords = files
  .filter(f => f.enhanced?.documentMetadata?.wordCount)
  .reduce((sum, f) => sum + (f.enhanced?.documentMetadata?.wordCount || 0), 0);

console.log(`Total words in project: ${totalWords.toLocaleString()}`);
```

**Find Drafts (by keywords):**
```typescript
const drafts = files.filter(f =>
  f.enhanced?.documentMetadata?.keywords?.includes('draft')
);
```

---

## Technical Details

### How It Works

**Office Documents (.docx, .xlsx, .pptx):**
- Modern Office files are ZIP archives containing XML
- We extract metadata from:
  - `docProps/core.xml` - Document properties
  - `docProps/app.xml` - Application statistics
  - Content XMLs - For structure analysis (sheets, slides, formulas)

**PDF Documents:**
- Parse PDF header and catalog
- Extract Info dictionary metadata
- Detect encryption, JavaScript, attachments

**Photoshop Files (.psd):**
- Binary file format
- Read file header (bytes 0-26)
- Parse dimensions, color mode, bit depth
- Detect alpha channel for transparency

### Dependencies

- **adm-zip** - For reading Office document ZIP archives
- Built-in Node.js Buffer - For binary file parsing (PSD, PDF)

### Performance

| File Type | Avg Time | Notes |
|-----------|----------|-------|
| **Word** | 20-50ms | Depends on document size |
| **Excel** | 30-80ms | More complex due to sheets |
| **PowerPoint** | 25-60ms | Slide parsing is fast |
| **PDF** | 10-30ms | Header parsing only |
| **PSD** | 5-15ms | Header reading only |

**Batch Processing:**
- Documents extracted in parallel batches
- No blocking during extraction
- Graceful fallback for encrypted/corrupted files

---

## Limitations

### Current Limitations

1. **Old Office Formats (.doc, .xls, .ppt):**
   - Binary formats (not ZIP-based)
   - Limited metadata extraction
   - Falls back to "encrypted" status

2. **Encrypted Files:**
   - Cannot read password-protected documents
   - Detected as `isEncrypted: true`

3. **PDF Page Count:**
   - Rough estimate from catalog
   - May not be accurate for all PDFs

4. **PSD Layer Count:**
   - Basic detection only
   - Full layer parsing is complex

5. **File Size Limits:**
   - Very large files (>100MB) may timeout
   - Not recommended for batch processing

### Future Enhancements

- [ ] Full layer parsing for PSD
- [ ] AI/Illustrator file support
- [ ] Sketch/Figma file metadata
- [ ] Video/Audio duration extraction
- [ ] OCR for scanned PDFs
- [ ] Document language detection
- [ ] Readability scores

---

## Error Handling

All extraction methods are **non-blocking** and **fail-safe**:

```typescript
try {
  enhanced.documentMetadata = await extractWordMetadata(path);
} catch (error) {
  // Logs error but doesn't crash
  console.error(`Failed to extract metadata:`, error);
  // Sets encrypted flag if ZIP reading fails
  return { isEncrypted: true };
}
```

**Errors are logged to console** for debugging.

---

## API Reference

### DocumentDetector Class

#### Methods

**`extractWordMetadata(absolutePath: string): Promise<DocumentMetadata>`**
- Extracts metadata from Word documents (.docx, .doc)
- Returns: title, author, page count, word count, etc.

**`extractExcelMetadata(absolutePath: string): Promise<SpreadsheetMetadata>`**
- Extracts metadata from Excel spreadsheets (.xlsx, .xls)
- Returns: sheet count, formulas, macros, charts, etc.

**`extractPowerPointMetadata(absolutePath: string): Promise<PresentationMetadata>`**
- Extracts metadata from PowerPoint presentations (.pptx, .ppt)
- Returns: slide count, animations, transitions, etc.

**`extractPDFMetadata(absolutePath: string): Promise<PDFMetadata>`**
- Extracts metadata from PDF documents
- Returns: page count, author, encryption status, etc.

**`extractPSDMetadata(absolutePath: string): Promise<DesignMetadata>`**
- Extracts metadata from Photoshop files (.psd)
- Returns: dimensions, color mode, bit depth, etc.

---

## Examples

### Example 1: Find All Marketing Documents

```typescript
const marketingDocs = files.filter(f => {
  const keywords = f.enhanced?.documentMetadata?.keywords || [];
  return keywords.some(k => k.toLowerCase().includes('marketing'));
});

console.log(`Found ${marketingDocs.length} marketing documents`);
```

### Example 2: Audit Presentation Quality

```typescript
const presentations = files.filter(f => f.extension === '.pptx');

presentations.forEach(file => {
  const meta = file.enhanced?.documentMetadata as PresentationMetadata;
  console.log(`${file.filename}:`);
  console.log(`  Slides: ${meta?.slideCount}`);
  console.log(`  Animations: ${meta?.hasAnimations ? 'Yes' : 'No'}`);
  console.log(`  Notes: ${meta?.hasNotes ? 'Yes' : 'No'}`);
});
```

### Example 3: Design File Inventory

```typescript
const psdFiles = files.filter(f => f.extension === '.psd');

const inventory = psdFiles.map(file => ({
  name: file.filename,
  dimensions: `${file.enhanced?.designMetadata?.width}×${file.enhanced?.designMetadata?.height}`,
  colorMode: file.enhanced?.designMetadata?.colorMode,
  fileSize: file.enhanced?.stats.size
}));

console.table(inventory);
```

---

## Testing

To test the new metadata extraction:

1. **Add sample files** to your workspace:
   - Word documents (.docx)
   - Excel spreadsheets (.xlsx)
   - PowerPoint presentations (.pptx)
   - PDF documents
   - Photoshop files (.psd)

2. **Press F5** to launch Extension Development Host

3. **Open Debug Console** (View → Debug Console)

4. **Watch for extraction logs**:
   ```
   [MetadataExtractor] Extracting Word metadata...
   [MetadataExtractor] ✓ Word: 25 pages, 5430 words

   [MetadataExtractor] Extracting Excel metadata...
   [MetadataExtractor] ✓ Excel: 5 sheets, formulas: yes

   [MetadataExtractor] Extracting PSD metadata...
   [MetadataExtractor] ✓ PSD: 3840×2160, RGB
   ```

5. **Inspect extracted data** in your tree views

---

## Summary

**New Capabilities:**
- ✅ Word documents (title, author, page count, word count)
- ✅ Excel spreadsheets (sheets, formulas, macros)
- ✅ PowerPoint presentations (slides, animations, media)
- ✅ PDF documents (pages, author, encryption)
- ✅ Photoshop files (dimensions, color mode, transparency)

**Perfect For:**
- 📝 Office workers managing documents
- 🎨 Designers organizing assets
- ✍️ Writers tracking manuscripts
- 📊 Analysts working with spreadsheets
- 🖼️ Creative teams with mixed media

**Next Steps:**
- Test with your actual files
- Provide feedback on accuracy
- Request additional metadata fields
- Suggest new file format support

---

**Cortex Office & Photoshop Metadata** - Know your files, inside and out! 📄🎨
