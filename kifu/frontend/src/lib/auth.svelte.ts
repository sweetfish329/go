// Simple Svelte 5 reactive auth store
class AuthStore {
  username = $state<string | null>(localStorage.getItem("kifu_username"));
  userId = $state<string | null>(localStorage.getItem("kifu_userid"));
  isLoaded = $state<boolean>(false);

  get isLoggedIn(): boolean {
    return this.userId !== null;
  }

  setLogin(username: string, userId: string): void {
    this.username = username;
    this.userId = userId;
    localStorage.setItem("kifu_username", username);
    localStorage.setItem("kifu_userid", userId);
  }

  async checkAuth(): Promise<void> {
    try {
      const res = await fetch("/api/auth/me");
      if (res.ok) {
        const user = await res.json();
        this.username = user.username;
        this.userId = user.id;
        localStorage.setItem("kifu_username", user.username);
        localStorage.setItem("kifu_userid", user.id);
      } else {
        this.clearAuth();
      }
    } catch (e) {
      console.error(e);
      this.clearAuth();
    } finally {
      this.isLoaded = true;
    }
  }

  async logout(): Promise<void> {
    try {
      await fetch("/api/auth/logout", { method: "POST" });
    } catch (e) {
      console.error(e);
    }
    this.clearAuth();
  }

  private clearAuth(): void {
    this.username = null;
    this.userId = null;
    localStorage.removeItem("kifu_username");
    localStorage.removeItem("kifu_userid");
  }

  getHeaders(): Record<string, string> {
    return {
      "Content-Type": "application/json",
    };
  }
}

export const auth = new AuthStore();
