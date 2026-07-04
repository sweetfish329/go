// Simple Svelte 5 reactive auth store
class AuthStore {
  token = $state<string | null>(localStorage.getItem('kifu_token'));
  username = $state<string | null>(localStorage.getItem('kifu_username'));
  userId = $state<string | null>(localStorage.getItem('kifu_userid'));

  get isLoggedIn(): boolean {
    return this.token !== null;
  }

  setLogin(token: string, username: string, userId: string): void {
    this.token = token;
    this.username = username;
    this.userId = userId;
    localStorage.setItem('kifu_token', token);
    localStorage.setItem('kifu_username', username);
    localStorage.setItem('kifu_userid', userId);
  }

  logout(): void {
    this.token = null;
    this.username = null;
    this.userId = null;
    localStorage.removeItem('kifu_token');
    localStorage.removeItem('kifu_username');
    localStorage.removeItem('kifu_userid');
  }

  getHeaders(): Record<string, string> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json'
    };
    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }
    return headers;
  }
}

export const auth = new AuthStore();
