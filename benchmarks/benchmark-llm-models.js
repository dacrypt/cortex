#!/usr/bin/env node
/**
 * LLM Model Benchmark Script for Cortex
 * Tests different models for speed, quality, and resource usage
 */

const ENDPOINT = 'http://localhost:11434';
const fs = require('fs');
const path = require('path');

// Set to true to only test models that are already installed (faster)
const ONLY_INSTALLED = process.argv.includes('--installed-only');

// Models to benchmark (ordered from smallest to largest)
const MODELS_TO_TEST = [
  // Small models (< 5B params)
  { name: 'phi3', size: '2.2GB', params: '3.8B', description: 'Very fast, lightweight' },
  { name: 'llama3.2', size: '2GB', params: '3.2B', description: 'Balanced, recommended' },

  // Medium models (7-8B params)
  { name: 'mistral', size: '4.1GB', params: '7B', description: 'High quality, medium speed' },
  { name: 'codellama', size: '3.8GB', params: '7B', description: 'Optimized for code' },
  { name: 'qwen2.5', size: '4.7GB', params: '7B', description: 'Strong instruction following' },
  { name: 'llama3.1', size: '4.9GB', params: '8B', description: 'Larger Llama baseline' },
  { name: 'deepseek-coder', size: '4.0GB', params: '7B', description: 'Code-focused model' },
  { name: 'gemma2', size: '5.4GB', params: '9B', description: 'Google Gemma 2' },

  // Large models (13-14B params)
  { name: 'codellama:13b', size: '7.4GB', params: '13B', description: 'CodeLlama 13B' },
  { name: 'qwen2.5:14b', size: '9.0GB', params: '14B', description: 'Qwen 2.5 14B' },
  { name: 'mistral-nemo', size: '7.1GB', params: '12B', description: 'Mistral Nemo 12B' },

  // Extra Large models (32B+ params) - will be slow!
  { name: 'qwen2.5:32b', size: '20GB', params: '32B', description: 'Qwen 2.5 32B - Large' },
  { name: 'codellama:34b', size: '19GB', params: '34B', description: 'CodeLlama 34B - Large' },
  { name: 'mixtral', size: '26GB', params: '8x7B', description: 'Mixtral MoE - Large' },
  { name: 'llama3.3', size: '43GB', params: '70B', description: 'Llama 3.3 70B - Very Large' },
  { name: 'qwen2.5:72b', size: '47GB', params: '72B', description: 'Qwen 2.5 72B - Very Large' },
];

// Test prompts (representing real Cortex use cases)
const TEST_PROMPTS = {
  tagSuggestion: {
    name: 'Tag Suggestion',
    prompt: `Analyze this TypeScript code and suggest 3-5 relevant tags:

/**
 * User authentication service
 * Handles login, logout, and token management
 */
export class AuthService {
  async login(email: string, password: string): Promise<User | null> {
    const token = await this.tokenService.generateToken(user.id);
    return user;
  }
}

Return ONLY a JSON array of lowercase tag strings.
Example: ["typescript", "authentication", "api"]`,
    expectedType: 'json_array'
  },

  strictJsonTags: {
    name: 'Strict JSON Tags',
    prompt: `Return ONLY a JSON array of 4 lowercase tags for this module:

Module: src/crypto/jwt.ts
Purpose: JWT signing, verification, token expiration, and refresh logic.

Response format: ["tag-one", "tag-two", "tag-three", "tag-four"]`,
    expectedType: 'json_array'
  },

  projectSuggestion: {
    name: 'Project Suggestion',
    prompt: `Based on these files, suggest a descriptive project name (2-3 words, lowercase with hyphens):

Files:
- src/auth/AuthService.ts
- src/auth/TokenService.ts
- src/models/User.ts

Return only the project name, nothing else.`,
    expectedType: 'string'
  },

  fileSummary: {
    name: 'File Summary',
    prompt: `Provide a concise one-sentence summary of this file's purpose:

File: src/components/UserProfile.tsx
Content: React component that displays user profile information including avatar, name, email, and bio.

Return only the summary (max 100 characters).`,
    expectedType: 'string'
  },

  streamSummary: {
    name: 'Stream Summary',
    prompt: `Summarize this file in one sentence (max 90 chars):

File: src/services/TagService.ts
Content: Service that computes AI-suggested tags, validates user selections, and persists them.`,
    expectedType: 'string',
    stream: true
  },

  longContextSummary: {
    name: 'Long Context Summary',
    prompt: `Summarize the file purpose in one sentence (max 120 chars).

File: src/core/MetadataStore.ts
Content: A TypeScript class that stores and retrieves metadata for files, supports
tags, summaries, project assignment, and provides a query API for filtering based
on tags, projects, timestamps, and file paths. It persists data to disk, handles
migrations, and exposes methods for importing/exporting metadata for backups.

Additional Context:
- Uses a JSON file as the backing store
- Supports optimistic updates and caching
- Ensures backward compatibility across versions
- Provides helper functions for fuzzy tag search and batch updates`,
    expectedType: 'string'
  }
};

// Colors for output
const colors = {
  reset: '\x1b[0m',
  bright: '\x1b[1m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  red: '\x1b[31m',
  cyan: '\x1b[36m',
};

function log(message, color = 'reset') {
  console.log(`${colors[color]}${message}${colors.reset}`);
}

function isJsonArray(text) {
  try {
    const trimmed = text.trim();
    const parsed = JSON.parse(trimmed);
    return Array.isArray(parsed) && parsed.every(item => typeof item === 'string');
  } catch {
    return false;
  }
}

function isSingleLineString(text) {
  const trimmed = text.trim();
  return trimmed.length > 0 && !trimmed.includes('\n');
}

function scoreCompliance(expectedType, response) {
  if (expectedType === 'json_array') return isJsonArray(response) ? 1 : 0;
  if (expectedType === 'string') return isSingleLineString(response) ? 1 : 0;
  return 0;
}

async function checkOllama() {
  try {
    const response = await fetch(`${ENDPOINT}/api/tags`);
    if (!response.ok) throw new Error('Not running');
    return true;
  } catch (error) {
    log('❌ Ollama is not running!', 'red');
    log('   Start it with: ollama serve', 'yellow');
    return false;
  }
}

async function getInstalledModels() {
  const response = await fetch(`${ENDPOINT}/api/tags`);
  const data = await response.json();
  return data.models.map(m => m.name.replace(':latest', ''));
}

async function pullModel(modelName) {
  log(`📦 Pulling ${modelName}...`, 'cyan');

  const response = await fetch(`${ENDPOINT}/api/pull`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name: modelName })
  });

  // Stream the response to show progress
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let lastStatus = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;

    const text = decoder.decode(value);
    const lines = text.split('\n').filter(l => l.trim());

    for (const line of lines) {
      try {
        const data = JSON.parse(line);
        if (data.status && data.status !== lastStatus) {
          process.stdout.write(`\r   ${data.status}${' '.repeat(50)}`);
          lastStatus = data.status;
        }
      } catch (e) {
        // Ignore parse errors
      }
    }
  }

  console.log(''); // New line
  log(`   ✅ ${modelName} downloaded`, 'green');
}

async function benchmarkModel(modelName, prompt, promptName, stream = false) {
  const startTime = Date.now();

  try {
    if (!stream) {
      const response = await fetch(`${ENDPOINT}/api/generate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          model: modelName,
          prompt: prompt,
          stream: false,
          options: {
            temperature: 0.3,
            num_predict: 200
          }
        })
      });

      const data = await response.json();
      const endTime = Date.now();
      const duration = (endTime - startTime) / 1000; // seconds

      return {
        success: true,
        response: data.response,
        duration: duration.toFixed(2),
        tokensPerSecond: (data.eval_count / (data.eval_duration / 1000000000)).toFixed(1)
      };
    }

    const response = await fetch(`${ENDPOINT}/api/generate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        model: modelName,
        prompt: prompt,
        stream: true,
        options: {
          temperature: 0.3,
          num_predict: 200
        }
      })
    });

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let firstTokenMs = null;
    let combined = '';
    let evalCount = 0;
    let evalDuration = 0;

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      if (firstTokenMs === null) {
        firstTokenMs = Date.now() - startTime;
      }

      const text = decoder.decode(value);
      const lines = text.split('\n').filter(l => l.trim());

      for (const line of lines) {
        try {
          const data = JSON.parse(line);
          if (data.response) combined += data.response;
          if (typeof data.eval_count === 'number') evalCount = data.eval_count;
          if (typeof data.eval_duration === 'number') evalDuration = data.eval_duration;
        } catch (e) {
          // Ignore parse errors
        }
      }
    }

    const endTime = Date.now();
    const duration = (endTime - startTime) / 1000; // seconds
    const tokensPerSecond = evalDuration
      ? (evalCount / (evalDuration / 1000000000)).toFixed(1)
      : '0.0';

    return {
      success: true,
      response: combined,
      duration: duration.toFixed(2),
      tokensPerSecond,
      firstTokenMs: firstTokenMs === null ? 'n/a' : firstTokenMs
    };
  } catch (error) {
    const endTime = Date.now();
    return {
      success: false,
      error: error.message,
      duration: ((endTime - startTime) / 1000).toFixed(2)
    };
  }
}

async function runBenchmark() {
  console.log('');
  log('═══════════════════════════════════════════════════════', 'bright');
  log('           🏎️  Cortex LLM Model Benchmark', 'bright');
  log('═══════════════════════════════════════════════════════', 'bright');
  console.log('');

  // Check Ollama
  log('1️⃣  Checking Ollama service...', 'blue');
  const isRunning = await checkOllama();
  if (!isRunning) process.exit(1);
  log('   ✅ Ollama is running', 'green');
  console.log('');

  // Get installed models
  const installedModels = await getInstalledModels();
  log(`2️⃣  Found ${installedModels.length} installed models: ${installedModels.join(', ')}`, 'blue');
  console.log('');

  // Download missing models
  log('3️⃣  Checking required models...', 'blue');
  for (const model of MODELS_TO_TEST) {
    if (!installedModels.includes(model.name)) {
      log(`   📥 Model "${model.name}" not found, downloading...`, 'yellow');
      await pullModel(model.name);
    } else {
      log(`   ✅ Model "${model.name}" already installed`, 'green');
    }
  }
  console.log('');

  // Run benchmarks
  log('4️⃣  Running benchmarks...', 'blue');
  console.log('');

  const results = {};

  for (const model of MODELS_TO_TEST) {
    log(`╔═══ Testing: ${model.name} (${model.size}, ${model.params}) ═══`, 'cyan');
    log(`║ ${model.description}`, 'cyan');
    log('╚═══════════════════════════════════════════════════', 'cyan');

    results[model.name] = {
      info: model,
      tests: {}
    };

    for (const [testKey, testData] of Object.entries(TEST_PROMPTS)) {
      process.stdout.write(`   Testing ${testData.name}... `);

      const result = await benchmarkModel(
        model.name,
        testData.prompt,
        testData.name,
        Boolean(testData.stream)
      );
      results[model.name].tests[testKey] = result;

      if (result.success) {
        const streamNote = result.firstTokenMs !== undefined ? ` | first token ${result.firstTokenMs}ms` : '';
        log(`✅ ${result.duration}s (${result.tokensPerSecond} tok/s)${streamNote}`, 'green');
        log(`   Response: ${result.response.substring(0, 80)}${result.response.length > 80 ? '...' : ''}`, 'reset');
      } else {
        log(`❌ Failed: ${result.error}`, 'red');
      }
    }
    console.log('');
  }

  // Generate summary report
  console.log('');
  log('═══════════════════════════════════════════════════════', 'bright');
  log('                  📊 Benchmark Results', 'bright');
  log('═══════════════════════════════════════════════════════', 'bright');
  console.log('');

  // Summary table
  console.log('┌─────────────┬──────────┬────────────┬──────────────┬─────────────┬──────────────┐');
  console.log('│ Model       │ Size     │ Avg Time   │ Tokens/sec   │ Quality     │ Compliance   │');
  console.log('├─────────────┼──────────┼────────────┼──────────────┼─────────────┼──────────────┤');

  const modelStats = [];

  for (const model of MODELS_TO_TEST) {
    const modelResults = results[model.name];
    if (!modelResults) continue;

    const successfulTests = Object.values(modelResults.tests).filter(t => t.success);

    if (successfulTests.length === 0) {
      console.log(`│ ${model.name.padEnd(11)} │ ${model.size.padEnd(8)} │ Failed     │ -            │ -           │ -            │`);
      continue;
    }

    const avgTime = (successfulTests.reduce((sum, t) => sum + parseFloat(t.duration), 0) / successfulTests.length).toFixed(2);
    const avgTokens = (successfulTests.reduce((sum, t) => sum + parseFloat(t.tokensPerSecond), 0) / successfulTests.length).toFixed(1);

    const complianceScores = Object.entries(modelResults.tests).map(([testKey, testResult]) => {
      if (!testResult.success) return 0;
      const expectedType = TEST_PROMPTS[testKey].expectedType;
      return scoreCompliance(expectedType, testResult.response);
    });
    const compliancePct = Math.round((complianceScores.reduce((sum, s) => sum + s, 0) / complianceScores.length) * 100);

    // Quality assessment (simple heuristic based on response length and content)
    let qualityScore = 0;
    successfulTests.forEach(test => {
      if (test.response.length > 10) qualityScore += 1;
      if (test.response.includes('[') || test.response.includes('{')) qualityScore += 1;
    });
    const quality = qualityScore >= 4 ? 'Excellent' : qualityScore >= 2 ? 'Good' : 'Fair';

    console.log(`│ ${model.name.padEnd(11)} │ ${model.size.padEnd(8)} │ ${avgTime.padEnd(10)} │ ${avgTokens.padEnd(12)} │ ${quality.padEnd(11)} │ ${String(compliancePct).padEnd(12)} │`);

    modelStats.push({
      name: model.name,
      avgTime: parseFloat(avgTime),
      avgTokens: parseFloat(avgTokens),
      quality,
      size: model.size,
      compliancePct
    });
  }

  console.log('└─────────────┴──────────┴────────────┴──────────────┴─────────────┴──────────────┘');
  console.log('');

  // Recommendations
  log('🎯 Recommendations:', 'bright');
  console.log('');

  // Find fastest
  const fastest = modelStats.sort((a, b) => a.avgTime - b.avgTime)[0];
  log(`⚡ Fastest: ${fastest.name} (${fastest.avgTime}s avg)`, 'green');
  log(`   Best for: Quick suggestions, real-time tagging`, 'reset');
  console.log('');

  // Find best quality
  const bestQuality = modelStats.filter(m => m.quality === 'Excellent')[0];
  if (bestQuality) {
    log(`🎨 Best Quality: ${bestQuality.name}`, 'green');
    log(`   Best for: Project analysis, detailed summaries`, 'reset');
    console.log('');
  }

  // Balanced recommendation
  const balanced = modelStats.find(m => m.avgTime < 5 && m.quality === 'Excellent') ||
                   modelStats.find(m => m.avgTime < 5 && m.quality === 'Good');
  if (balanced) {
    log(`⚖️  Recommended (Balanced): ${balanced.name}`, 'yellow');
    log(`   Speed: ${balanced.avgTime}s | Quality: ${balanced.quality}`, 'reset');
    log(`   Good balance of speed and quality for general use`, 'reset');
    console.log('');
  }

  // Configuration suggestion
  log('📝 Suggested Configuration:', 'bright');
  console.log('');
  const recommended = balanced || fastest;
  console.log(`{
  "cortex.llm.enabled": true,
  "cortex.llm.endpoint": "http://localhost:11434",
  "cortex.llm.model": "${recommended.name}",
  "cortex.llm.maxContextTokens": 2000
}`);
  console.log('');

  log('✅ Benchmark complete!', 'green');
  console.log('');

  const timestamp = new Date().toISOString().replace(/[:]/g, '').replace(/\..+/, '').replace('T', '_');
  const outputDir = path.join(process.cwd(), 'benchmarks');
  fs.mkdirSync(outputDir, { recursive: true });

  const jsonPath = path.join(outputDir, `benchmark-results-${timestamp}.json`);
  fs.writeFileSync(jsonPath, JSON.stringify({ timestamp, results }, null, 2));

  const csvPath = path.join(outputDir, `benchmark-results-${timestamp}.csv`);
  const csvRows = [
    ['model', 'size', 'avg_time_s', 'tokens_per_sec', 'quality', 'compliance_pct'].join(',')
  ];
  modelStats.forEach(stat => {
    csvRows.push([stat.name, stat.size, stat.avgTime, stat.avgTokens, stat.quality, stat.compliancePct].join(','));
  });
  fs.writeFileSync(csvPath, `${csvRows.join('\n')}\n`);

  log(`📄 JSON report saved: ${jsonPath}`, 'cyan');
  log(`📄 CSV report saved: ${csvPath}`, 'cyan');
}

runBenchmark().catch(error => {
  log(`\n❌ Benchmark failed: ${error.message}`, 'red');
  console.error(error);
  process.exit(1);
});
