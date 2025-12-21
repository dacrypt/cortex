/**
 * GoExecutor - Utility to execute Go code from VSCode extension
 * 
 * This demonstrates how to run Go code from a TypeScript extension.
 * You can either:
 * 1. Execute a compiled Go binary
 * 2. Run `go run` to execute Go source files directly
 * 3. Use `go build` to compile and then execute
 */

import { execFile, exec } from 'child_process';
import { promisify } from 'util';
import * as path from 'path';

const execFileAsync = promisify(execFile);
const execAsync = promisify(exec);

export interface GoExecutionResult {
  stdout: string;
  stderr: string;
  exitCode: number;
}

/**
 * Execute a compiled Go binary
 * @param binaryPath Path to the compiled Go binary
 * @param args Arguments to pass to the binary
 * @returns Execution result
 */
export async function executeGoBinary(
  binaryPath: string,
  args: string[] = []
): Promise<GoExecutionResult> {
  try {
    const { stdout, stderr } = await execFileAsync(binaryPath, args);
    return {
      stdout: stdout.toString(),
      stderr: stderr.toString(),
      exitCode: 0,
    };
  } catch (error: any) {
    return {
      stdout: error.stdout?.toString() || '',
      stderr: error.stderr?.toString() || error.message || '',
      exitCode: error.code || 1,
    };
  }
}

/**
 * Execute Go source file directly using `go run`
 * @param goFilePath Path to the .go source file
 * @param args Arguments to pass to the Go program
 * @returns Execution result
 */
export async function runGoFile(
  goFilePath: string,
  args: string[] = []
): Promise<GoExecutionResult> {
  try {
    const command = `go run "${goFilePath}" ${args.map(a => `"${a}"`).join(' ')}`;
    const { stdout, stderr } = await execAsync(command, {
      cwd: path.dirname(goFilePath),
    });
    return {
      stdout: stdout.toString(),
      stderr: stderr.toString(),
      exitCode: 0,
    };
  } catch (error: any) {
    return {
      stdout: error.stdout?.toString() || '',
      stderr: error.stderr?.toString() || error.message || '',
      exitCode: error.code || 1,
    };
  }
}

/**
 * Compile a Go source file and execute the binary
 * @param goFilePath Path to the .go source file
 * @param outputPath Optional path for the compiled binary (defaults to temp file)
 * @param args Arguments to pass to the compiled binary
 * @returns Execution result
 */
export async function buildAndRunGoFile(
  goFilePath: string,
  outputPath?: string,
  args: string[] = []
): Promise<GoExecutionResult> {
  const goDir = path.dirname(goFilePath);
  const goFileName = path.basename(goFilePath, '.go');
  const binaryPath = outputPath || path.join(goDir, goFileName);

  try {
    // First, compile the Go file
    const buildCommand = `go build -o "${binaryPath}" "${goFilePath}"`;
    await execAsync(buildCommand, { cwd: goDir });

    // Then execute the binary
    return await executeGoBinary(binaryPath, args);
  } catch (error: any) {
    return {
      stdout: error.stdout?.toString() || '',
      stderr: error.stderr?.toString() || error.message || '',
      exitCode: error.code || 1,
    };
  }
}

/**
 * Execute a Go command (like `go fmt`, `go test`, etc.)
 * @param command Go command to execute (e.g., 'fmt', 'test', 'build')
 * @param workingDir Working directory for the command
 * @param args Additional arguments
 * @returns Execution result
 */
export async function executeGoCommand(
  command: string,
  workingDir: string,
  args: string[] = []
): Promise<GoExecutionResult> {
  try {
    const fullCommand = `go ${command} ${args.join(' ')}`;
    const { stdout, stderr } = await execAsync(fullCommand, {
      cwd: workingDir,
    });
    return {
      stdout: stdout.toString(),
      stderr: stderr.toString(),
      exitCode: 0,
    };
  } catch (error: any) {
    return {
      stdout: error.stdout?.toString() || '',
      stderr: error.stderr?.toString() || error.message || '',
      exitCode: error.code || 1,
    };
  }
}

/**
 * Check if Go is installed and available
 * @returns true if Go is available, false otherwise
 */
export async function isGoAvailable(): Promise<boolean> {
  try {
    await execFileAsync('go', ['version']);
    return true;
  } catch {
    return false;
  }
}


