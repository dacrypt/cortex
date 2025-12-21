#!/usr/bin/env node
/**
 * Quick test script to verify LLM integration
 */

async function testLLM() {
  console.log('🧪 Testing Cortex LLM Integration\n');

  // Test 1: Check Ollama is running
  console.log('1️⃣  Checking Ollama service...');
  try {
    const response = await fetch('http://localhost:11434/api/tags');
    const data = await response.json();
    console.log('   ✅ Ollama is running');
    console.log(`   📦 Models: ${data.models.map(m => m.name).join(', ')}\n`);
  } catch (error) {
    console.log('   ❌ Ollama not running. Start it with: ollama serve\n');
    process.exit(1);
  }

  // Test 2: Test tag suggestion
  console.log('2️⃣  Testing AI tag suggestion...');
  const tagPrompt = `Analyze this TypeScript code and suggest 3-5 relevant tags:

/**
 * User authentication service
 * Handles login, logout, and token management
 */

import { User } from './models/User';
import { TokenService } from './services/TokenService';

export class AuthService {
  async login(email: string, password: string): Promise<User | null> {
    // Validate credentials and generate JWT token
    const token = await this.tokenService.generateToken(user.id);
    return user;
  }
}

Return ONLY a JSON array of lowercase tag strings.
Example: ["typescript", "api", "authentication"]`;

  try {
    const response = await fetch('http://localhost:11434/api/generate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        model: 'llama3.2',
        prompt: tagPrompt,
        stream: false,
        options: {
          temperature: 0.3,
          num_predict: 200
        }
      })
    });

    const data = await response.json();
    console.log('   ✅ Tag suggestion response:', data.response);

    // Try to parse the JSON
    try {
      const tags = JSON.parse(data.response.match(/\[.*\]/)[0]);
      console.log('   🏷️  Suggested tags:', tags.join(', '));
    } catch (e) {
      console.log('   ⚠️  Could not parse as JSON, but got response');
    }
    console.log('');
  } catch (error) {
    console.log('   ❌ Error:', error.message, '\n');
  }

  // Test 3: Test project suggestion
  console.log('3️⃣  Testing AI project suggestion...');
  const projectPrompt = `Based on this file, suggest a descriptive project name (2-3 words, lowercase with hyphens):

File: src/auth/AuthService.ts
Content: User authentication service with JWT token management

Return only the project name, nothing else.`;

  try {
    const response = await fetch('http://localhost:11434/api/generate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        model: 'llama3.2',
        prompt: projectPrompt,
        stream: false,
        options: {
          temperature: 0.3,
          num_predict: 50
        }
      })
    });

    const data = await response.json();
    const projectName = data.response.trim().toLowerCase();
    console.log('   ✅ Project suggestion:', projectName);
    console.log('');
  } catch (error) {
    console.log('   ❌ Error:', error.message, '\n');
  }

  // Test 4: Test file summary
  console.log('4️⃣  Testing AI file summary...');
  const summaryPrompt = `Provide a concise one-sentence summary of this file's purpose:

File: src/auth/AuthService.ts
Content: User authentication service that handles login, logout, and JWT token management

Return only the summary (max 100 characters).`;

  try {
    const response = await fetch('http://localhost:11434/api/generate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        model: 'llama3.2',
        prompt: summaryPrompt,
        stream: false,
        options: {
          temperature: 0.3,
          num_predict: 100
        }
      })
    });

    const data = await response.json();
    console.log('   ✅ Summary:', data.response.trim());
    console.log('');
  } catch (error) {
    console.log('   ❌ Error:', error.message, '\n');
  }

  console.log('🎉 All tests completed!\n');
  console.log('Next steps:');
  console.log('  1. Press F5 in VS Code to launch the extension');
  console.log('  2. Open test-llm-demo.ts');
  console.log('  3. Right-click → "Cortex: Suggest Tags (AI)"');
}

testLLM().catch(console.error);
