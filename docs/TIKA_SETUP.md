# Apache Tika Setup

Apache Tika Server provides comprehensive metadata extraction, text extraction, and language detection for 1000+ file types.

## Automatic Management (Recommended)

Cortex can automatically manage the Tika Server lifecycle - no manual setup required!

### Prerequisites

1. **Java must be installed** (required for Tika Server):
   ```bash
   # Check if Java is installed
   java -version
   
   # Install Java if needed:
   # macOS: brew install openjdk
   # Linux: sudo apt install openjdk-17-jdk
   # Windows: Download from https://adoptium.net/
   ```

2. **Tika Server JAR** (one of the following):
   - Auto-detected from system installation (Homebrew, system packages)
   - Download manually and set `jar_path` in configuration
   - Or use external Tika Server (set `manage_process: false`)

### Configuration

Enable automatic Tika management in `cortexd.yaml`:

```yaml
tika:
  enabled: true
  manage_process: true  # Cortex will start/stop Tika automatically
  jar_path: ""  # Leave empty for auto-detection, or set path to tika-server-standard.jar
  endpoint: "http://localhost:9998"
  port: 9998
  timeout: 30s
  max_file_size: 104857600  # 100MB
  startup_timeout: 30s
  health_interval: 10s
  max_restarts: 3
  restart_delay: 5s
```

**That's it!** Cortex will:
- ✅ Automatically find and start Tika Server on startup
- ✅ Monitor Tika Server health
- ✅ Restart Tika if it crashes (up to `max_restarts` times)
- ✅ Stop Tika cleanly when Cortex shuts down

## Manual Management (Alternative)

If you prefer to manage Tika Server yourself:

### Option 1: Docker

```bash
docker run -d -p 9998:9998 --name cortex-tika apache/tika:latest
```

Or use the provided `docker-compose.yml`:

```bash
docker-compose up -d tika
```

### Option 2: JAR Direct

```bash
# Download tika-server-standard.jar
wget https://archive.apache.org/dist/tika/tika-server-standard-2.9.1.jar

# Run Tika Server
java -jar tika-server-standard-2.9.1.jar --host=0.0.0.0 --port=9998
```

### Option 3: Homebrew (macOS)

```bash
brew install tika
tika-server
```

Then configure Cortex to use external Tika:

```yaml
tika:
  enabled: true
  manage_process: false  # Don't manage Tika - use external instance
  endpoint: "http://localhost:9998"
  timeout: 30s
  max_file_size: 104857600
```

### Configuration Options

- `enabled`: Enable/disable Tika extraction (default: `false`)
- `manage_process`: Auto-start/stop Tika Server (default: `true`)
- `jar_path`: Path to tika-server-standard.jar (empty = auto-detect)
- `endpoint`: Tika Server URL (default: `http://localhost:9998`)
- `port`: Port for Tika Server (default: `9998`)
- `timeout`: Request timeout (default: `30s`)
- `max_file_size`: Maximum file size to process (default: `100MB`)
- `startup_timeout`: Timeout for Tika to start (default: `30s`)
- `health_interval`: Interval for health checks (default: `10s`)
- `max_restarts`: Max restart attempts if Tika crashes (default: `3`)
- `restart_delay`: Delay before restarting (default: `5s`)

## Verification

Test that Tika Server is running:

```bash
curl http://localhost:9998/tika
```

You should receive a response indicating Tika is available.

## Features

Tika Server provides:

1. **MIME Type Detection**: Accurate content-type detection using magic bytes
2. **Metadata Extraction**: Rich metadata from PDFs, Office documents, images, audio, video, etc.
3. **Text Extraction**: Plain text extraction from documents
4. **Language Detection**: Automatic language detection from content

## Supported File Types

Tika supports 1000+ file types including:

- **Documents**: PDF, Word (doc, docx), Excel (xls, xlsx), PowerPoint (ppt, pptx), OpenDocument formats
- **Images**: JPEG, PNG, GIF, TIFF, WebP, HEIC, RAW formats
- **Audio**: MP3, FLAC, OGG, WAV, AAC, M4A, etc.
- **Video**: MP4, AVI, MKV, MOV, WebM, etc.
- **Archives**: ZIP, TAR, GZ, BZ2, 7Z, RAR, etc.
- **Code**: Source code files (with basic metadata)
- **And many more...**

## Automatic Lifecycle Management

When `manage_process: true`, Cortex automatically:

1. **Finds or Downloads Tika JAR**: Searches common paths, then downloads if `auto_download: true`
2. **Starts Tika**: Launches Tika Server as a subprocess on Cortex startup
3. **Monitors Health**: Periodically checks Tika Server availability
4. **Auto-Restarts**: Restarts Tika if it crashes (up to `max_restarts` times)
5. **Clean Shutdown**: Stops Tika gracefully when Cortex shuts down

### Finding and Downloading Tika JAR

Cortex searches for Tika in this order:
1. Path specified in `jar_path` configuration
2. System-wide locations (`/usr/share/tika`, `/opt/tika`, etc.)
3. Homebrew locations (`/opt/homebrew/Cellar/tika`, `/usr/local/Cellar/tika`)
4. Current directory (`./tika-server-standard.jar`)
5. `tika-server` command wrapper script
6. **Auto-download** (if `auto_download: true`): Downloads from Apache Maven repository to `{data_dir}/tika/tika-server-standard.jar`

### Auto-Download

When `auto_download: true` and Tika JAR is not found:
- Cortex automatically downloads the latest stable version from Apache Maven
- JAR is stored in `{data_dir}/tika/tika-server-standard.jar`
- Download happens once - subsequent starts reuse the cached JAR
- Requires internet connection for first download

If download fails or `auto_download: false`, Cortex will use fallback extractors.

## Fallback Behavior

If Tika Server is not available or fails:

1. Cortex will automatically fall back to existing extractors (PDF, Image, Audio, Video extractors)
2. No indexing errors will occur - the pipeline continues with available extractors
3. Logs will indicate when Tika is unavailable
4. If `manage_process: true` and Tika fails to start, Cortex will continue without Tika

## Performance Considerations

- **First Request**: Tika may be slow on first request (JVM warmup)
- **Large Files**: Files > 100MB may timeout - adjust `timeout` and `max_file_size` accordingly
- **Concurrent Requests**: Tika Server handles multiple concurrent requests efficiently

## Troubleshooting

### Tika Server Not Available

If you see warnings about Tika Server not being available:

1. Check that Tika is running: `curl http://localhost:9998/tika`
2. Verify the endpoint in `cortexd.yaml` matches your Tika Server URL
3. Check firewall/network settings if Tika is on a different host

### Timeout Errors

If you see timeout errors:

1. Increase `timeout` in configuration (e.g., `60s` or `120s`)
2. Reduce `max_file_size` to skip very large files
3. Check Tika Server logs for processing issues

### Memory Issues

Tika Server (Java) can be memory-intensive:

1. Increase Docker container memory limits if using Docker
2. Set JVM options: `-Xmx2g` for 2GB heap
3. Monitor Tika Server memory usage

## Integration

Tika is integrated into the Cortex indexing pipeline:

1. **Primary Extractor**: When enabled, Tika is tried first for all file types
2. **Fallback**: If Tika fails or is disabled, existing extractors are used
3. **Metadata Merging**: Tika metadata is merged with existing metadata extraction

## API Endpoints

Tika Server exposes these endpoints (used internally by Cortex):

- `PUT /tika` - Detect MIME type
- `PUT /meta` - Extract metadata (JSON)
- `PUT /tika` - Extract text
- `PUT /language/stream` - Detect language

For more information, see [Apache Tika Server documentation](https://tika.apache.org/2.9.1/server.html).

