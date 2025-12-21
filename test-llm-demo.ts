/**
 * User authentication service
 * Handles login, logout, and token management
 */

import { User } from './models/User';
import { TokenService } from './services/TokenService';

export class AuthService {
  private tokenService: TokenService;

  constructor() {
    this.tokenService = new TokenService();
  }

  /**
   * Authenticate user with email and password
   */
  async login(email: string, password: string): Promise<User | null> {
    // Validate input
    if (!email || !password) {
      throw new Error('Email and password are required');
    }

    // Check credentials against database
    const user = await this.validateCredentials(email, password);

    if (!user) {
      return null;
    }

    // Generate JWT token
    const token = await this.tokenService.generateToken(user.id);

    // Store token in session
    user.authToken = token;

    return user;
  }

  /**
   * Validate user credentials
   */
  private async validateCredentials(email: string, password: string): Promise<User | null> {
    // TODO: Implement actual database lookup
    return null;
  }

  /**
   * Logout user and invalidate token
   */
  async logout(userId: string): Promise<void> {
    await this.tokenService.invalidateToken(userId);
  }
}
