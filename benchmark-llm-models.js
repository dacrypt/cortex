#!/usr/bin/env node
/**
 * LLM Model Benchmark Script for Cortex
 * Tests different models for speed, quality, and resource usage
 */

const ENDPOINT = 'http://localhost:11434';

// Models to benchmark (ordered from smallest to largest)
const MODELS_TO_TEST = [
  { name: 'phi3', size: '3.8GB', params: '3.8B', description: 'Very fast, lightweight' },
  { name: 'llama3.2', size: '2GB', params: '3.2B', description: 'Balanced, recommended' },
  { name: 'mistral', size: '4.1GB', params: '7B', description: 'High quality, medium speed' },
  { name: 'codellama', size: '3.8GB', params: '7B', description: 'Optimized for code' },
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

async function benchmarkModel(modelName, prompt, promptName) {
  const startTime = Date.now();

  try {
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

      const result = await benchmarkModel(model.name, testData.prompt, testData.name);
      results[model.name].tests[testKey] = result;

      if (result.success) {
        log(`✅ ${result.duration}s (${result.tokensPerSecond} tok/s)`, 'green');
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
  console.log('┌─────────────┬──────────┬────────────┬──────────────┬─────────────┐');
  console.log('│ Model       │ Size     │ Avg Time   │ Tokens/sec   │ Quality     │');
  console.log('├─────────────┼──────────┼────────────┼──────────────┼─────────────┤');

  const modelStats = [];

  for (const model of MODELS_TO_TEST) {
    const modelResults = results[model.name];
    if (!modelResults) continue;

    const successfulTests = Object.values(modelResults.tests).filter(t => t.success);

    if (successfulTests.length === 0) {
      console.log(`│ ${model.name.padEnd(11)} │ ${model.size.padEnd(8)} │ Failed     │ -            │ -           │`);
      continue;
    }

    const avgTime = (successfulTests.reduce((sum, t) => sum + parseFloat(t.duration), 0) / successfulTests.length).toFixed(2);
    const avgTokens = (successfulTests.reduce((sum, t) => sum + parseFloat(t.tokensPerSecond), 0) / successfulTests.length).toFixed(1);

    // Quality assessment (simple heuristic based on response length and content)
    let qualityScore = 0;
    successfulTests.forEach(test => {
      if (test.response.length > 10) qualityScore += 1;
      if (test.response.includes('[') || test.response.includes('{')) qualityScore += 1;
    });
    const quality = qualityScore >= 4 ? 'Excellent' : qualityScore >= 2 ? 'Good' : 'Fair';

    console.log(`│ ${model.name.padEnd(11)} │ ${model.size.padEnd(8)} │ ${avgTime.padEnd(10)} │ ${avgTokens.padEnd(12)} │ ${quality.padEnd(11)} │`);

    modelStats.push({
      name: model.name,
      avgTime: parseFloat(avgTime),
      avgTokens: parseFloat(avgTokens),
      quality,
      size: model.size
    });
  }

  console.log('└─────────────┴──────────┴────────────┴──────────────┴─────────────┘');
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
}

runBenchmark().catch(error => {
  log(`\n❌ Benchmark failed: ${error.message}`, 'red');
  console.error(error);
  process.exit(1);
});
